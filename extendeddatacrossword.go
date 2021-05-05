package rsmt2d

import (
	"bytes"
	"errors"
	"fmt"
	"math"
)

const (
	row = iota
	col
)

// ErrUnrepairableDataSquare is thrown when there is insufficient chunks to repair the square.
var ErrUnrepairableDataSquare = errors.New("failed to solve data square")

// ErrByzantineRow is thrown when a repaired row does not match the expected row Merkle root.
type ErrByzantineRow struct {
	RowNumber uint     // Row index
	Shares    [][]byte // Pre-repaired row shares. Missing shares are nil.
}

func (e *ErrByzantineRow) Error() string {
	return fmt.Sprintf("byzantine row: %d", e.RowNumber)
}

// ErrByzantineCol is thrown when a repaired column does not match the expected column Merkle root.
type ErrByzantineCol struct {
	ColNumber uint     // Column index
	Shares    [][]byte // Pre-repaired column shares. Missing shares are nil.
}

func (e *ErrByzantineCol) Error() string {
	return fmt.Sprintf("byzantine column: %d", e.ColNumber)
}

// RepairExtendedDataSquare attempts to repair an incomplete extended data
// square (EDS), comparing repaired rows and columns against expected Merkle
// roots.
//
// Input
//
// Missing shares must be nil.
//
// Output
//
// The EDS is modified in-place. If repairing is successful, the EDS will be
// complete. If repairing is unsuccessful, the EDS will be the most-repaired
// prior to the Byzantine row or column being repaired, and the Byzantine row
// or column prior to repair is returned in the error with missing shares as
// nil.
func RepairExtendedDataSquare(
	rowRoots [][]byte,
	colRoots [][]byte,
	data [][]byte,
	codec Codec,
	treeCreatorFn TreeConstructorFn,
) (*ExtendedDataSquare, error) {
	width := int(math.Ceil(math.Sqrt(float64(len(data)))))
	bitMat := newBitMatrix(width)
	var chunkSize int
	for i := range data {
		if data[i] != nil {
			bitMat.SetFlat(i)
			if chunkSize == 0 {
				chunkSize = len(data[i])
			}
		}
	}

	if chunkSize == 0 {
		return nil, ErrUnrepairableDataSquare
	}

	fillerChunk := bytes.Repeat([]byte{0}, chunkSize)
	for i := range data {
		if data[i] == nil {
			data[i] = make([]byte, chunkSize)
			copy(data[i], fillerChunk)
		}
	}

	eds, err := ImportExtendedDataSquare(data, codec, treeCreatorFn)
	if err != nil {
		return nil, err
	}

	err = eds.prerepairSanityCheck(rowRoots, colRoots, bitMat, codec)
	if err != nil {
		return nil, err
	}

	err = eds.solveCrossword(rowRoots, colRoots, bitMat, codec)
	if err != nil {
		return nil, err
	}

	return eds, err
}

// solveCrossword attempts to iteratively repair an EDS.
func (eds *ExtendedDataSquare) solveCrossword(
	rowRoots [][]byte,
	colRoots [][]byte,
	bitMask bitMatrix,
	codec Codec,
) error {
	// Keep repeating until the square is solved
	for {
		// Track if the entire square is completely solved
		solved := true
		// Track if a single iteration of this loop made progress
		progressMade := false

		// Loop through every row and column, attempt to rebuild each row or column if incomplete
		for i := 0; i < int(eds.width); i++ {
			solvedRow, progressMadeRow, err := eds.solveCrosswordRow(i, rowRoots, colRoots, bitMask, codec)
			if err != nil {
				return err
			}
			solvedCol, progressMadeCol, err := eds.solveCrosswordCol(i, rowRoots, colRoots, bitMask, codec)
			if err != nil {
				return err
			}

			solved = solved && solvedRow && solvedCol
			progressMade = progressMade || progressMadeRow || progressMadeCol
		}

		if solved {
			break
		}
		if !progressMade {
			return ErrUnrepairableDataSquare
		}
	}

	return nil
}

// solveCrosswordRow attempts to repair a single row.
// Returns
// - if the row is solved (i.e. complete)
// - if the row was previously unsolved and now solved
// - an error if the repair is unsuccessful
func (eds *ExtendedDataSquare) solveCrosswordRow(
	r int,
	rowRoots [][]byte,
	colRoots [][]byte,
	bitMask bitMatrix,
	codec Codec,
) (bool, bool, error) {
	isComplete := bitMask.RowIsOne(r)
	if isComplete {
		return true, false, nil
	}

	// Prepare shares
	shares := make([][]byte, eds.width)
	for c := 0; c < int(eds.width); c++ {
		vectorData := eds.row(uint(r))

		if bitMask.Get(r, c) {
			// As guaranteed by the bitMask, vectorData can't be nil here:
			shares[c] = vectorData[c]
		}
	}

	isExtendedPartIncomplete := !bitMask.RowRangeIsOne(r, int(eds.originalDataWidth), int(eds.width))
	// Attempt rebuild
	rebuiltShares, isDecoded, err := eds.rebuildShares(isExtendedPartIncomplete, shares, codec)
	if err != nil {
		return false, false, err
	}
	if !isDecoded {
		return false, false, nil
	}

	// Check that rebuilt shares matches appropriate root
	err = eds.verifyAgainstRowRoots(rowRoots, uint(r), bitMask, rebuiltShares)
	if err != nil {
		return false, false, err
	}

	// Check that newly completed orthogonal vectors match their new merkle roots
	for c := 0; c < int(eds.width); c++ {
		if !bitMask.Get(r, c) &&
			bitMask.ColIsOne(c) {
			err := eds.verifyAgainstColRoots(colRoots, uint(c), bitMask, rebuiltShares)
			if err != nil {
				return false, false, err
			}
		}
	}

	// Set vector mask to true
	for c := 0; c < int(eds.width); c++ {
		bitMask.Set(r, c)
	}

	// Insert rebuilt shares into square.
	for c, s := range rebuiltShares {
		eds.setCell(uint(r), uint(c), s)
	}

	return true, true, nil
}

// solveCrosswordCol attempts to repair a single column.
// Returns
// - if the column is solved (i.e. complete)
// - if the column was previously unsolved and now solved
// - an error if the repair is unsuccessful
func (eds *ExtendedDataSquare) solveCrosswordCol(
	c int,
	rowRoots [][]byte,
	colRoots [][]byte,
	bitMask bitMatrix,
	codec Codec,
) (bool, bool, error) {
	isComplete := bitMask.ColIsOne(c)
	if isComplete {
		return true, false, nil
	}

	// Prepare shares
	shares := make([][]byte, eds.width)
	for r := 0; r < int(eds.width); r++ {
		vectorData := eds.col(uint(c))

		if bitMask.Get(r, c) {
			// As guaranteed by the bitMask, vectorData can't be nil here:
			shares[r] = vectorData[r]
		}
	}

	isExtendedPartIncomplete := !bitMask.ColRangeIsOne(c, int(eds.originalDataWidth), int(eds.width))
	// Attempt rebuild
	rebuiltShares, isDecoded, err := eds.rebuildShares(isExtendedPartIncomplete, shares, codec)
	if err != nil {
		return false, false, err
	}
	if !isDecoded {
		return false, false, nil
	}

	// Check that rebuilt shares matches appropriate root
	err = eds.verifyAgainstColRoots(colRoots, uint(c), bitMask, rebuiltShares)
	if err != nil {
		return false, false, err
	}

	// Check that newly completed orthogonal vectors match their new merkle roots
	for r := 0; r < int(eds.width); r++ {
		if !bitMask.Get(r, c) &&
			bitMask.RowIsOne(r) {
			err := eds.verifyAgainstRowRoots(rowRoots, uint(r), bitMask, rebuiltShares)
			if err != nil {
				return false, false, err
			}
		}
	}

	// Set vector mask to true
	for r := 0; r < int(eds.width); r++ {
		bitMask.Set(r, c)
	}

	// Insert rebuilt shares into square.
	for r, s := range rebuiltShares {
		eds.setCell(uint(r), uint(c), s)
	}

	return true, true, nil
}

func (eds *ExtendedDataSquare) rebuildShares(
	isExtendedPartIncomplete bool,
	shares [][]byte,
	codec Codec,
) ([][]byte, bool, error) {
	rebuiltShares, err := codec.Decode(shares)
	if err != nil {
		// repair unsuccessful
		return nil, false, nil
	}

	if isExtendedPartIncomplete {
		// If needed, rebuild the parity shares too.
		rebuiltExtendedShares, err := codec.Encode(rebuiltShares[0:eds.originalDataWidth])
		if err != nil {
			return nil, true, err
		}
		startIndex := len(rebuiltExtendedShares) - int(eds.originalDataWidth)
		rebuiltShares = append(
			rebuiltShares[0:eds.originalDataWidth],
			rebuiltExtendedShares[startIndex:]...,
		)
	} else {
		// Otherwise copy them from the EDS.
		startIndex := len(shares) - int(eds.originalDataWidth)
		rebuiltShares = append(
			rebuiltShares[0:eds.originalDataWidth],
			shares[startIndex:]...,
		)
	}

	return rebuiltShares, true, nil
}

func (eds *ExtendedDataSquare) verifyAgainstRowRoots(
	rowRoots [][]byte,
	r uint,
	bitMask bitMatrix,
	shares [][]byte,
) error {
	root := eds.computeSharesRoot(shares, r)

	if !bytes.Equal(root, rowRoots[r]) {
		for c := 0; c < int(eds.width); c++ {
			if !bitMask.Get(int(r), c) {
				shares[c] = nil
			}
		}
		return &ErrByzantineRow{r, shares}
	}

	return nil
}

func (eds *ExtendedDataSquare) verifyAgainstColRoots(
	colRoots [][]byte,
	c uint, bitMask bitMatrix,
	shares [][]byte,
) error {
	root := eds.computeSharesRoot(shares, c)

	if !bytes.Equal(root, colRoots[c]) {
		for r := 0; r < int(eds.width); r++ {
			if !bitMask.Get(r, int(c)) {
				shares[r] = nil
			}
		}
		return &ErrByzantineCol{c, shares}
	}

	return nil
}

func (eds *ExtendedDataSquare) prerepairSanityCheck(
	rowRoots [][]byte,
	colRoots [][]byte,
	bitMask bitMatrix,
	codec Codec,
) error {
	for i := uint(0); i < eds.width; i++ {
		rowIsComplete := bitMask.RowIsOne(int(i))
		colIsComplete := bitMask.ColIsOne(int(i))

		// if there's no missing data in the this row
		if noMissingData(eds.row(i)) {
			// ensure that the roots are equal and that rowMask is a vector
			if rowIsComplete && !bytes.Equal(rowRoots[i], eds.getRowRoot(i)) {
				return fmt.Errorf("bad root input: row %d expected %v got %v", i, rowRoots[i], eds.getRowRoot(i))
			}
		}

		// if there's no missing data in the this col
		if noMissingData(eds.col(i)) {
			// ensure that the roots are equal and that rowMask is a vector
			if colIsComplete && !bytes.Equal(colRoots[i], eds.getColRoot(i)) {
				return fmt.Errorf("bad root input: col %d expected %v got %v", i, colRoots[i], eds.getColRoot(i))
			}
		}

		if rowIsComplete {
			parityShares, err := codec.Encode(eds.rowSlice(i, 0, eds.originalDataWidth))
			if err != nil {
				return err
			}
			if !bytes.Equal(flattenChunks(parityShares), flattenChunks(eds.rowSlice(i, eds.originalDataWidth, eds.originalDataWidth))) {
				return &ErrByzantineRow{i, eds.row(i)}
			}
		}

		if colIsComplete {
			parityShares, err := codec.Encode(eds.colSlice(0, i, eds.originalDataWidth))
			if err != nil {
				return err
			}
			if !bytes.Equal(flattenChunks(parityShares), flattenChunks(eds.colSlice(eds.originalDataWidth, i, eds.originalDataWidth))) {
				return &ErrByzantineCol{i, eds.col(i)}
			}
		}
	}

	return nil
}

func noMissingData(input [][]byte) bool {
	for _, d := range input {
		if d == nil {
			return false
		}
	}
	return true
}

func (eds *ExtendedDataSquare) computeSharesRoot(shares [][]byte, i uint) []byte {
	tree := eds.createTreeFn()
	for cell, d := range shares {
		tree.Push(d, SquareIndex{Cell: uint(cell), Axis: i})
	}
	return tree.Root()
}
