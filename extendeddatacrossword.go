package rsmt2d

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// Axis represents which of a row or col.
type Axis int

const (
	Row Axis = iota
	Col
)

const (
	// noShareInsertion indicates that a new share hasn't been inserted in the eds
	noShareInsertion = -1
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

// ErrByzantineData is returned when a repaired row or column does not match the
// expected row or column Merkle root. It is also returned when the parity data
// from a row or a column is not equal to the encoded original data.
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
// # Input
//
// Missing shares must be nil.
//
// # Output
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
	isComplete := noMissingData(eds.row(uint(r)), noShareInsertion)
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
	err = eds.verifyAgainstRowRoots(rowRoots, uint(r), rebuiltShares, noShareInsertion, nil)
	if err != nil {
		var byzErr *ErrByzantineData
		if errors.As(err, &byzErr) {
			byzErr.Shares = shares
		}
		return false, false, err
	}

	// Check that newly completed orthogonal vectors match their new merkle roots
	for c := 0; c < int(eds.width); c++ {
		col := eds.col(uint(c))
		if col[r] != nil {
			continue // not newly completed
		}
		if noMissingData(col, r) { // not completed
			err := eds.verifyAgainstColRoots(colRoots, uint(c), col, r, rebuiltShares[c])
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
	isComplete := noMissingData(eds.col(uint(c)), noShareInsertion)
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
	err = eds.verifyAgainstColRoots(colRoots, uint(c), rebuiltShares, noShareInsertion, nil)
	if err != nil {
		var byzErr *ErrByzantineData
		if errors.As(err, &byzErr) {
			byzErr.Shares = shares
		}
		return false, false, err
	}

	// Check that newly completed orthogonal vectors match their new merkle roots
	for r := 0; r < int(eds.width); r++ {
		row := eds.row(uint(r))
		if row[c] != nil {
			continue // not newly completed
		}
		if noMissingData(row, c) { // not completed
			err := eds.verifyAgainstRowRoots(rowRoots, uint(r), row, c, rebuiltShares[r])
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
	oldShares [][]byte,
	rebuiltIndex int,
	rebuiltShare []byte,
) error {
	var root []byte
	var err error
	if rebuiltIndex < 0 || rebuiltShare == nil {
		root, err = eds.computeSharesRoot(oldShares, Row, r)
	} else {
		root, err = eds.computeSharesRootWithRebuiltShare(oldShares, Row, r, rebuiltIndex, rebuiltShare)
	}
	if err != nil {
		return err
	}

	if !bytes.Equal(root, rowRoots[r]) {
		return &ErrByzantineData{Row, r, nil}
	}

	return nil
}

func (eds *ExtendedDataSquare) verifyAgainstColRoots(
	colRoots [][]byte,
	c uint,
	oldShares [][]byte,
	rebuiltIndex int,
	rebuiltShare []byte,
) error {
	var root []byte
	var err error
	if rebuiltIndex < 0 || rebuiltShare == nil {
		root, err = eds.computeSharesRoot(oldShares, Col, c)
	} else {
		root, err = eds.computeSharesRootWithRebuiltShare(oldShares, Col, c, rebuiltIndex, rebuiltShare)
	}
	if err != nil {
		return err
	}

	if !bytes.Equal(root, colRoots[c]) {
		return &ErrByzantineData{Col, c, nil}
	}

	return nil
}

func (eds *ExtendedDataSquare) prerepairSanityCheck(
	rowRoots [][]byte,
	colRoots [][]byte,
) error {
	errs, _ := errgroup.WithContext(context.Background())

	for i := uint(0); i < eds.width; i++ {
		i := i

		rowIsComplete := noMissingData(eds.row(i), noShareInsertion)
		// if there's no missing data in this row
		if rowIsComplete {
			errs.Go(func() error {
				// ensure that the roots are equal
				rowRoot, err := eds.getRowRoot(i)
				if err != nil {
					return err
				}
				if !bytes.Equal(rowRoots[i], rowRoot) {
					return fmt.Errorf("bad root input: row %d expected %v got %v", i, rowRoots[i], rowRoot)
				}
				return nil
			})
			errs.Go(func() error {
				parityShares, err := eds.codec.Encode(eds.rowSlice(i, 0, eds.originalDataWidth))
				if err != nil {
					return err
				}
				if !bytes.Equal(flattenChunks(parityShares), flattenChunks(eds.rowSlice(i, eds.originalDataWidth, eds.originalDataWidth))) {
					return &ErrByzantineData{Row, i, eds.row(i)}
				}
				return nil
			})
		}

		colIsComplete := noMissingData(eds.col(i), noShareInsertion)
		// if there's no missing data in this col
		if colIsComplete {
			errs.Go(func() error {
				// ensure that the roots are equal
				colRoot, err := eds.getColRoot(i)
				if err != nil {
					return err
				}
				if !bytes.Equal(colRoots[i], colRoot) {
					return fmt.Errorf("bad root input: col %d expected %v got %v", i, colRoots[i], colRoot)
				}
				return nil
			})
			errs.Go(func() error {
				parityShares, err := eds.codec.Encode(eds.colSlice(0, i, eds.originalDataWidth))
				if err != nil {
					return err
				}
				if !bytes.Equal(flattenChunks(parityShares), flattenChunks(eds.colSlice(eds.originalDataWidth, i, eds.originalDataWidth))) {
					return &ErrByzantineData{Col, i, eds.col(i)}
				}
				return nil
			})
		}
	}

	return errs.Wait()
}

func noMissingData(input [][]byte, rebuiltIndex int) bool {
	for index, d := range input {
		if index == rebuiltIndex {
			continue
		}
		if d == nil {
			return false
		}
	}
	return true
}

func (eds *ExtendedDataSquare) computeSharesRoot(shares [][]byte, axis Axis, i uint) ([]byte, error) {
	tree := eds.createTreeFn(axis, i)
	for _, d := range shares {
		tree.Push(d)
	}
	return tree.Root()
}

func (eds *ExtendedDataSquare) computeSharesRootWithRebuiltShare(shares [][]byte, axis Axis, i uint, rebuiltIndex int, rebuiltShare []byte) ([]byte, error) {
	tree := eds.createTreeFn(axis, i)
	for _, d := range shares[:rebuiltIndex] {
		tree.Push(d)
	}
	tree.Push(rebuiltShare)
	for _, d := range shares[rebuiltIndex+1:] {
		tree.Push(d)
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
