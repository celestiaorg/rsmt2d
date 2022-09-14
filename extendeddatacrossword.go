package rsmt2d

import (
	"bytes"
	"errors"
	"fmt"
)

// Axis represents which of a row or col.
type Axis int

const (
	Row Axis = iota
	Col
)

func (a Axis) String() string {
	switch a {
	case Row:
		return "row"
	case Col:
		return "col"
	default:
		panic(fmt.Sprintf("invalid axis type: %d", a))
	}
}

// ErrUnrepairableDataSquare is thrown when there is insufficient chunks to repair the square.
var ErrUnrepairableDataSquare = errors.New("failed to solve data square")

// ErrByzantineData is thrown when a repaired row or column does not match the expected row or column Merkle root.
type ErrByzantineData struct {
	Axis   Axis     // Axis of the data.
	Index  uint     // Row/Col index.
	Shares [][]byte // Pre-repaired shares. Missing shares are nil.
}

func (e *ErrByzantineData) Error() string {
	return fmt.Sprintf("byzantine %s: %d", e.Axis, e.Index)
}

// Repair attempts to repair an incomplete extended data
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
func (eds *ExtendedDataSquare) Repair(
	rowRoots [][]byte,
	colRoots [][]byte,
) error {
	err := eds.prerepairSanityCheck(rowRoots, colRoots)
	if err != nil {
		return err
	}

	return eds.solveCrossword(rowRoots, colRoots)
}

// solveCrossword attempts to iteratively repair an EDS.
func (eds *ExtendedDataSquare) solveCrossword(
	rowRoots [][]byte,
	colRoots [][]byte,
) error {
	// Keep repeating until the square is solved
	for {
		// Track if the entire square is completely solved
		solved := true
		// Track if a single iteration of this loop made progress
		progressMade := false

		// Loop through every row and column, attempt to rebuild each row or column if incomplete
		for i := 0; i < int(eds.width); i++ {
			solvedRow, progressMadeRow, err := eds.solveCrosswordRow(i, rowRoots, colRoots)
			if err != nil {
				return err
			}
			solvedCol, progressMadeCol, err := eds.solveCrosswordCol(i, rowRoots, colRoots)
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
) (bool, bool, error) {
	isComplete := noMissingData(eds.row(uint(r)))
	if isComplete {
		return true, false, nil
	}

	// Prepare shares
	shares := make([][]byte, eds.width)
	vectorData := eds.row(uint(r))
	for c := 0; c < int(eds.width); c++ {
		shares[c] = vectorData[c]
	}

	isExtendedPartIncomplete := !eds.rowRangeNoMissingData(uint(r), eds.originalDataWidth, eds.width)
	// Attempt rebuild
	rebuiltShares, isDecoded, err := eds.rebuildShares(isExtendedPartIncomplete, shares)
	if err != nil {
		return false, false, err
	}
	if !isDecoded {
		return false, false, nil
	}

	// Check that rebuilt shares matches appropriate root
	err = eds.verifyAgainstRowRoots(rowRoots, uint(r), rebuiltShares)
	if err != nil {
		return false, false, err
	}

	// Check that newly completed orthogonal vectors match their new merkle roots
	for c := 0; c < int(eds.width); c++ {
		col := eds.col(uint(c))
		if col[r] != nil {
			continue // not newly completed
		}
		col[r] = rebuiltShares[c]
		if noMissingData(col) { // not completed
			err := eds.verifyAgainstColRoots(colRoots, uint(c), col)
			if err != nil {
				return false, false, err
			}
		}
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
) (bool, bool, error) {
	isComplete := noMissingData(eds.col(uint(c)))
	if isComplete {
		return true, false, nil
	}

	// Prepare shares
	shares := make([][]byte, eds.width)
	vectorData := eds.col(uint(c))
	for r := 0; r < int(eds.width); r++ {
		shares[r] = vectorData[r]

	}

	isExtendedPartIncomplete := !eds.colRangeNoMissingData(uint(c), eds.originalDataWidth, eds.width)
	// Attempt rebuild
	rebuiltShares, isDecoded, err := eds.rebuildShares(isExtendedPartIncomplete, shares)
	if err != nil {
		return false, false, err
	}
	if !isDecoded {
		return false, false, nil
	}

	// Check that rebuilt shares matches appropriate root
	err = eds.verifyAgainstColRoots(colRoots, uint(c), rebuiltShares)
	if err != nil {
		return false, false, err
	}

	// Check that newly completed orthogonal vectors match their new merkle roots
	for r := 0; r < int(eds.width); r++ {
		row := eds.row(uint(r))
		if row[c] != nil {
			continue // not newly completed
		}
		row[c] = rebuiltShares[r]
		if noMissingData(row) { // not completed
			err := eds.verifyAgainstRowRoots(rowRoots, uint(r), row)
			if err != nil {
				return false, false, err
			}
		}
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
) ([][]byte, bool, error) {
	rebuiltShares, err := eds.codec.Decode(shares)
	if err != nil {
		// repair unsuccessful
		return nil, false, nil
	}

	if isExtendedPartIncomplete {
		// If needed, rebuild the parity shares too.
		rebuiltExtendedShares, err := eds.codec.Encode(rebuiltShares[0:eds.originalDataWidth])
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
	shares [][]byte,
) error {
	root := eds.computeSharesRoot(shares, r)

	if !bytes.Equal(root, rowRoots[r]) {
		return &ErrByzantineData{Row, r, shares}
	}

	return nil
}

func (eds *ExtendedDataSquare) verifyAgainstColRoots(
	colRoots [][]byte,
	c uint,
	shares [][]byte,
) error {
	root := eds.computeSharesRoot(shares, c)

	if !bytes.Equal(root, colRoots[c]) {
		return &ErrByzantineData{Col, c, shares}
	}

	return nil
}

func (eds *ExtendedDataSquare) prerepairSanityCheck(
	rowRoots [][]byte,
	colRoots [][]byte,
) error {
	for i := uint(0); i < eds.width; i++ {
		rowIsComplete := noMissingData(eds.row(i))
		colIsComplete := noMissingData(eds.col(i))

		// if there's no missing data in the this row
		if rowIsComplete {
			// ensure that the roots are equal and that rowMask is a vector
			if !bytes.Equal(rowRoots[i], eds.getRowRoot(i)) {
				return fmt.Errorf("bad root input: row %d expected %v got %v", i, rowRoots[i], eds.getRowRoot(i))
			}
		}

		// if there's no missing data in the this col
		if colIsComplete {
			// ensure that the roots are equal and that rowMask is a vector
			if !bytes.Equal(colRoots[i], eds.getColRoot(i)) {
				return fmt.Errorf("bad root input: col %d expected %v got %v", i, colRoots[i], eds.getColRoot(i))
			}
		}

		if rowIsComplete {
			parityShares, err := eds.codec.Encode(eds.rowSlice(i, 0, eds.originalDataWidth))
			if err != nil {
				return err
			}
			if !bytes.Equal(flattenChunks(parityShares), flattenChunks(eds.rowSlice(i, eds.originalDataWidth, eds.originalDataWidth))) {
				return &ErrByzantineData{Row, i, eds.row(i)}
			}
		}

		if colIsComplete {
			parityShares, err := eds.codec.Encode(eds.colSlice(0, i, eds.originalDataWidth))
			if err != nil {
				return err
			}
			if !bytes.Equal(flattenChunks(parityShares), flattenChunks(eds.colSlice(eds.originalDataWidth, i, eds.originalDataWidth))) {
				return &ErrByzantineData{Col, i, eds.col(i)}
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

func (eds *ExtendedDataSquare) rowRangeNoMissingData(r, start, end uint) bool {
	for c := start; c <= end && c < eds.width; c++ {
		if eds.squareRow[r][c] == nil {
			return false
		}
	}
	return true
}

func (eds *ExtendedDataSquare) colRangeNoMissingData(c, start, end uint) bool {
	for r := start; r <= end && r < eds.width; r++ {
		if eds.squareRow[r][c] == nil {
			return false
		}
	}
	return true
}
