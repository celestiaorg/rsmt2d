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

// ErrUnrepairableDataSquare is thrown when there is insufficient shares to repair the square.
var ErrUnrepairableDataSquare = errors.New("failed to solve data square")

// ErrByzantineData is returned when a repaired row or column does not match the
// expected row or column Merkle root. It is also returned when the parity data
// from a row or a column is not equal to the encoded original data.
type ErrByzantineData struct {
	// Axis describes if this ErrByzantineData is for a row or column.
	Axis Axis
	// Index is the row or column index.
	Index uint
	// Shares contain the shares in the row or column that the client can
	// determine proofs for (either through sampling or using shares decoded
	// from the extended data square). In other words, it contains shares whose
	// individual inclusion is guaranteed to be provable by the full node (i.e.
	// shares usable in a bad encoding fraud proof). Missing shares are nil.
	Shares [][]byte
}

func (e *ErrByzantineData) Error() string {
	return fmt.Sprintf(
		"byzantine %s: %d", e.Axis, e.Index)
}

// Repair attempts to repair an incomplete extended data square (EDS). The
// parameters rowRoots and colRoots are the expected Merkle roots for each row
// and column. rowRoots and colRoots are used to verify that a repaired row or
// column is correct. Prior to the repair process, if a row or column is already
// complete but the Merkle root for the row or column doesn't match the expected
// root, an error is returned. Missing shares in the EDS must be nil.
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
	err := eds.preRepairSanityCheck(rowRoots, colRoots)
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
	rowIdx int,
	rowRoots [][]byte,
	colRoots [][]byte,
) (bool, bool, error) {
	isComplete := noMissingData(eds.row(uint(rowIdx)), noShareInsertion)
	if isComplete {
		return true, false, nil
	}

	// Prepare shares
	shares := make([][]byte, eds.width)
	copy(shares, eds.row(uint(rowIdx)))

	// Attempt rebuild the row
	rebuiltShares, isDecoded, err := eds.rebuildShares(shares)
	if err != nil {
		return false, false, err
	}
	if !isDecoded {
		return false, false, nil
	}

	// Check that rebuilt shares matches appropriate root
	err = eds.verifyAgainstRowRoots(rowRoots, uint(rowIdx), rebuiltShares, noShareInsertion, nil)
	if err != nil {
		var byzErr *ErrByzantineData
		if errors.As(err, &byzErr) {
			byzErr.Shares = shares
			return false, false, byzErr
		}
		return false, false, err
	}

	// Check that newly completed orthogonal vectors match their new merkle roots
	for colIdx := 0; colIdx < int(eds.width); colIdx++ {
		col := eds.col(uint(colIdx))
		if col[rowIdx] != nil {
			continue // not newly completed
		}
		if noMissingData(col, rowIdx) { // completed
			err := eds.verifyAgainstColRoots(colRoots, uint(colIdx), col, rowIdx, rebuiltShares[colIdx])
			if err != nil {
				var byzErr *ErrByzantineData
				if errors.As(err, &byzErr) {
					byzErr.Shares = shares
				}
				return false, false, err
			}

			if eds.verifyEncoding(col, rowIdx, rebuiltShares[colIdx]) != nil {
				return false, false, &ErrByzantineData{Col, uint(colIdx), col}
			}
		}
	}

	// Insert rebuilt shares into square.
	for colIdx, s := range rebuiltShares {
		cellToSet := eds.GetCell(uint(rowIdx), uint(colIdx))
		if cellToSet == nil {
			err := eds.SetCell(uint(rowIdx), uint(colIdx), s)
			if err != nil {
				return false, false, err
			}
		}
	}

	return true, true, nil
}

// solveCrosswordCol attempts to repair a single column.
// Returns
// - if the column is solved (i.e. complete)
// - if the column was previously unsolved and now solved
// - an error if the repair is unsuccessful
func (eds *ExtendedDataSquare) solveCrosswordCol(
	colIdx int,
	rowRoots [][]byte,
	colRoots [][]byte,
) (bool, bool, error) {
	isComplete := noMissingData(eds.col(uint(colIdx)), noShareInsertion)
	if isComplete {
		return true, false, nil
	}

	// Prepare shares
	shares := make([][]byte, eds.width)
	copy(shares, eds.col(uint(colIdx)))

	// Attempt rebuild
	rebuiltShares, isDecoded, err := eds.rebuildShares(shares)
	if err != nil {
		return false, false, err
	}
	if !isDecoded {
		return false, false, nil
	}

	// Check that rebuilt shares matches appropriate root
	err = eds.verifyAgainstColRoots(colRoots, uint(colIdx), rebuiltShares, noShareInsertion, nil)
	if err != nil {
		var byzErr *ErrByzantineData
		if errors.As(err, &byzErr) {
			byzErr.Shares = shares
			return false, false, byzErr
		}
		return false, false, err
	}

	// Check that newly completed orthogonal vectors match their new merkle roots
	for rowIdx := 0; rowIdx < int(eds.width); rowIdx++ {
		row := eds.row(uint(rowIdx))
		if row[colIdx] != nil {
			continue // not newly completed
		}
		if noMissingData(row, colIdx) { // completed
			err := eds.verifyAgainstRowRoots(rowRoots, uint(rowIdx), row, colIdx, rebuiltShares[rowIdx])
			if err != nil {
				var byzErr *ErrByzantineData
				if errors.As(err, &byzErr) {
					byzErr.Shares = shares
				}
				return false, false, err
			}

			if eds.verifyEncoding(row, colIdx, rebuiltShares[rowIdx]) != nil {
				return false, false, &ErrByzantineData{Row, uint(rowIdx), row}
			}
		}
	}

	// Insert rebuilt shares into square.
	for rowIdx, s := range rebuiltShares {
		cellToSet := eds.GetCell(uint(rowIdx), uint(colIdx))
		if cellToSet == nil {
			err := eds.SetCell(uint(rowIdx), uint(colIdx), s)
			if err != nil {
				return false, false, err
			}
		}
	}

	return true, true, nil
}

// rebuildShares attempts to rebuild a row or column of shares.
// Returns
// 1. An entire row or column of shares so original + parity shares.
// 2. Whether the original shares could be decoded from the shares parameter.
// 3. [Optional] an error.
func (eds *ExtendedDataSquare) rebuildShares(
	shares [][]byte,
) ([][]byte, bool, error) {
	rebuiltShares, err := eds.codec.Decode(shares)
	if err != nil {
		// Decode was unsuccessful but don't propagate the error because that
		// would halt the progress of solveCrosswordRow or solveCrosswordCol.
		return nil, false, nil
	}

	return rebuiltShares, true, nil
}

func (eds *ExtendedDataSquare) verifyAgainstRowRoots(
	rowRoots [][]byte,
	rowIdx uint,
	oldShares [][]byte,
	rebuiltIndex int,
	rebuiltShare []byte,
) error {
	var root []byte
	var err error
	if rebuiltIndex < 0 || rebuiltShare == nil {
		root, err = eds.computeSharesRoot(oldShares, Row, rowIdx)
	} else {
		root, err = eds.computeSharesRootWithRebuiltShare(oldShares, Row, rowIdx, rebuiltIndex, rebuiltShare)
	}
	if err != nil {
		// any error during the computation of the root is considered byzantine
		// the shares are set to nil, as the caller will populate them
		return &ErrByzantineData{Row, rowIdx, nil}
	}

	if !bytes.Equal(root, rowRoots[rowIdx]) {
		// the shares are set to nil, as the caller will populate them
		return &ErrByzantineData{Row, rowIdx, nil}
	}

	return nil
}

// verifyAgainstColRoots checks that the shares of column index `c` match their expected column root available in `colRoots`.
// `colRoots` is a slice of the expected roots of the columns of the `eds`.
// `shares` is a slice of the shares of the column index `c` of the `eds`.
// `rebuiltIndex` is the index of the share that was rebuilt, if any.
// `rebuiltShare` is the rebuilt share, if any.
// Returns a ErrByzantineData error if the computed root does not match the expected root or if the root computation fails.
func (eds *ExtendedDataSquare) verifyAgainstColRoots(
	colRoots [][]byte,
	colIdx uint,
	shares [][]byte,
	rebuiltIndex int,
	rebuiltShare []byte,
) error {
	var root []byte
	var err error
	if rebuiltIndex < 0 || rebuiltShare == nil {
		root, err = eds.computeSharesRoot(shares, Col, colIdx)
	} else {
		root, err = eds.computeSharesRootWithRebuiltShare(shares, Col, colIdx, rebuiltIndex, rebuiltShare)
	}
	if err != nil {
		// the shares are set to nil, as the caller will populate them
		return &ErrByzantineData{Col, colIdx, nil}
	}

	if !bytes.Equal(root, colRoots[colIdx]) {
		// the shares are set to nil, as the caller will populate them
		return &ErrByzantineData{Col, colIdx, nil}
	}

	return nil
}

// preRepairSanityCheck returns an error if any row or column in the EDS is
// complete and the computed Merkle root for that row or column doesn't match
// the given root in rowRoots or colRoots.
func (eds *ExtendedDataSquare) preRepairSanityCheck(
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
					// any error regarding the root calculation signifies an issue in the shares e.g., out of order shares
					// therefore, it should be treated as byzantine data
					return &ErrByzantineData{Row, i, eds.row(i)}
				}
				if !bytes.Equal(rowRoots[i], rowRoot) {
					// if the roots are not equal, then the data is byzantine
					return &ErrByzantineData{Row, i, eds.row(i)}
				}
				return nil
			})
			errs.Go(func() error {
				err := eds.verifyEncoding(eds.row(i), noShareInsertion, nil)
				if err != nil {
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
					// any error regarding the root calculation signifies an issue in the shares e.g., out of order shares
					// therefore, it should be treated as byzantine data
					return &ErrByzantineData{Col, i, eds.col(i)}
				}
				if !bytes.Equal(colRoots[i], colRoot) {
					// if the roots are not equal, then the data is byzantine
					return &ErrByzantineData{Col, i, eds.col(i)}
				}
				return nil
			})
			errs.Go(func() error {
				err := eds.verifyEncoding(eds.col(i), noShareInsertion, nil)
				if err != nil {
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

// computeSharesRoot calculates the root of the shares for the specified axis (`i`th column or row).
func (eds *ExtendedDataSquare) computeSharesRoot(shares [][]byte, axis Axis, i uint) ([]byte, error) {
	tree := eds.createTreeFn(axis, i)
	for _, d := range shares {
		err := tree.Push(d)
		if err != nil {
			return nil, err
		}
	}
	return tree.Root()
}

// computeSharesRootWithRebuiltShare computes the root of the shares with the rebuilt share `rebuiltShare` at the specified index `rebuiltIndex`.
func (eds *ExtendedDataSquare) computeSharesRootWithRebuiltShare(shares [][]byte, axis Axis, i uint, rebuiltIndex int, rebuiltShare []byte) ([]byte, error) {
	tree := eds.createTreeFn(axis, i)
	for _, d := range shares[:rebuiltIndex] {
		err := tree.Push(d)
		if err != nil {
			return nil, err
		}
	}

	err := tree.Push(rebuiltShare)
	if err != nil {
		return nil, err
	}

	for _, d := range shares[rebuiltIndex+1:] {
		err := tree.Push(d)
		if err != nil {
			return nil, err
		}
	}
	return tree.Root()
}

// verifyEncoding checks the Reed-Solomon encoding of the provided data.
func (eds *ExtendedDataSquare) verifyEncoding(data [][]byte, rebuiltIndex int, rebuiltShare []byte) error {
	if rebuiltShare != nil && rebuiltIndex >= 0 {
		data[rebuiltIndex] = rebuiltShare
		defer func() {
			// revert the change to the data slice after the verification
			data[rebuiltIndex] = nil
		}()
	}

	half := len(data) / 2
	original := data[:half]
	parity, err := eds.codec.Encode(original)
	if err != nil {
		return err
	}

	for i := half; i < len(data); i++ {
		if !bytes.Equal(data[i], parity[i-half]) {
			return errors.New("parity data does not match encoded data")
		}
	}
	return nil
}
