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
	// noChunkInsertion indicates that a new chunk hasn't been inserted in the EDS
	noChunkInsertion = -1
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
	// Axis describes if this ErrByzantineData is for a row or column.
	Axis Axis
	// Index is the row or column index.
	Index uint
	// Shares (a.k.a chunks) contain the shares in the row or column that the
	// client can determine proofs for (either through sampling or using shares
	// decoded from the extended data square). In other words, it contains
	// shares whose individual inclusion is guaranteed to be provable by the
	// full node (i.e. shares usable in a bad encoding fraud proof). Missing
	// shares are nil.
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
// root, an error is returned. Missing chunks in the EDS must be nil.
//
// # Output
//
// The EDS is modified in-place. If repairing is successful, the EDS will be
// complete. If repairing is unsuccessful, the EDS will be the most-repaired
// prior to the Byzantine row or column being repaired, and the Byzantine row
// or column prior to repair is returned in the error with missing chunks as
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
	r int,
	rowRoots [][]byte,
	colRoots [][]byte,
) (bool, bool, error) {
	isComplete := noMissingData(eds.row(uint(r)), noChunkInsertion)
	if isComplete {
		return true, false, nil
	}

	// Prepare chunks
	chunks := make([][]byte, eds.width)
	vectorData := eds.row(uint(r))
	for c := 0; c < int(eds.width); c++ {
		chunks[c] = vectorData[c]
	}

	// Attempt rebuild the row
	rebuiltChunks, isDecoded, err := eds.rebuildChunks(chunks)
	if err != nil {
		return false, false, err
	}
	if !isDecoded {
		return false, false, nil
	}

	// Check that rebuilt chunks matches appropriate root
	err = eds.verifyAgainstRowRoots(rowRoots, uint(r), rebuiltChunks, noChunkInsertion, nil)
	if err != nil {
		var byzErr *ErrByzantineData
		if errors.As(err, &byzErr) {
			byzErr.Shares = chunks
		}
		return false, false, err
	}

	// Check that newly completed orthogonal vectors match their new merkle roots
	for c := 0; c < int(eds.width); c++ {
		col := eds.col(uint(c))
		if col[r] != nil {
			continue // not newly completed
		}
		if noMissingData(col, r) { // completed
			err := eds.verifyAgainstColRoots(colRoots, uint(c), col, r, rebuiltChunks[c])
			if err != nil {
				var byzErr *ErrByzantineData
				if errors.As(err, &byzErr) {
					byzErr.Shares = chunks
				}
				return false, false, err
			}
		}
	}

	// Insert rebuilt chunks into square.
	for c, s := range rebuiltChunks {
		cellToSet := eds.GetCell(uint(r), uint(c))
		if cellToSet == nil {
			err := eds.SetCell(uint(r), uint(c), s)
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
	c int,
	rowRoots [][]byte,
	colRoots [][]byte,
) (bool, bool, error) {
	isComplete := noMissingData(eds.col(uint(c)), noChunkInsertion)
	if isComplete {
		return true, false, nil
	}

	// Prepare chunks
	chunks := make([][]byte, eds.width)
	vectorData := eds.col(uint(c))
	for r := 0; r < int(eds.width); r++ {
		chunks[r] = vectorData[r]
	}

	// Attempt rebuild
	rebuiltChunks, isDecoded, err := eds.rebuildChunks(chunks)
	if err != nil {
		return false, false, err
	}
	if !isDecoded {
		return false, false, nil
	}

	// Check that rebuilt chunks matches appropriate root
	err = eds.verifyAgainstColRoots(colRoots, uint(c), rebuiltChunks, noChunkInsertion, nil)
	if err != nil {
		var byzErr *ErrByzantineData
		if errors.As(err, &byzErr) {
			byzErr.Shares = chunks
		}
		return false, false, err
	}

	// Check that newly completed orthogonal vectors match their new merkle roots
	for r := 0; r < int(eds.width); r++ {
		row := eds.row(uint(r))
		if row[c] != nil {
			continue // not newly completed
		}
		if noMissingData(row, c) { // completed
			err := eds.verifyAgainstRowRoots(rowRoots, uint(r), row, c, rebuiltChunks[r])
			if err != nil {
				var byzErr *ErrByzantineData
				if errors.As(err, &byzErr) {
					byzErr.Shares = chunks
				}
				return false, false, err
			}
		}
	}

	// Insert rebuilt chunks into square.
	for r, s := range rebuiltChunks {
		cellToSet := eds.GetCell(uint(r), uint(c))
		if cellToSet == nil {
			err := eds.SetCell(uint(r), uint(c), s)
			if err != nil {
				return false, false, err
			}
		}
	}

	return true, true, nil
}

// rebuildChunks attempts to rebuild a row or column of chunks.
// Returns
// 1. An entire row or column of chunks so original + parity chunks.
// 2. Whether the original chunks could be decoded from the chunks parameter.
// 3. [Optional] an error.
func (eds *ExtendedDataSquare) rebuildChunks(chunks [][]byte) ([][]byte, bool, error) {
	rebuiltChunks, err := eds.codec.Decode(chunks)
	if err != nil {
		// Decode was unsuccessful but don't propagate the error because that
		// would halt the progress of solveCrosswordRow or solveCrosswordCol.
		return nil, false, nil
	}

	return rebuiltChunks, true, nil
}

func (eds *ExtendedDataSquare) verifyAgainstRowRoots(
	rowRoots [][]byte,
	r uint,
	oldChunks [][]byte,
	rebuiltIndex int,
	rebuiltChunk []byte,
) error {
	var root []byte
	var err error
	if rebuiltIndex < 0 || rebuiltChunk == nil {
		root, err = eds.computeChunksRoot(oldChunks, Row, r)
	} else {
		root, err = eds.computeChunksRootWithRebuiltChunk(oldChunks, Row, r, rebuiltIndex, rebuiltChunk)
	}
	if err != nil {
		// any error during the computation of the root is considered byzantine
		// the shares are set to nil, as the caller will populate them
		return &ErrByzantineData{Row, r, nil}
	}

	if !bytes.Equal(root, rowRoots[r]) {
		// the shares are set to nil, as the caller will populate them
		return &ErrByzantineData{Row, r, nil}
	}

	return nil
}

// verifyAgainstColRoots checks that the chunks of columnIndex match their expected column root in colRoots.
// colRoots is a slice of the expected eds column roots.
// chunks is a slice of the chunks of the columnIndex of the eds.
// rebuiltIndex is the index of the chunk that was rebuilt, if any.
// rebuiltChunk is the rebuilt chunk, if any.
// Returns a ErrByzantineData error if the computed root does not match the expected root or if the root computation fails.
func (eds *ExtendedDataSquare) verifyAgainstColRoots(
	colRoots [][]byte,
	columnIndex uint,
	chunks [][]byte,
	rebuiltIndex int,
	rebuiltChunk []byte,
) error {
	var root []byte
	var err error
	if rebuiltIndex < 0 || rebuiltChunk == nil {
		root, err = eds.computeChunksRoot(chunks, Col, columnIndex)
	} else {
		root, err = eds.computeChunksRootWithRebuiltChunk(chunks, Col, columnIndex, rebuiltIndex, rebuiltChunk)
	}
	if err != nil {
		// the shares are set to nil, as the caller will populate them
		return &ErrByzantineData{Col, columnIndex, nil}
	}

	if !bytes.Equal(root, colRoots[columnIndex]) {
		// the shares are set to nil, as the caller will populate them
		return &ErrByzantineData{Col, columnIndex, nil}
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

		rowIsComplete := noMissingData(eds.row(i), noChunkInsertion)
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
				parityChunks, err := eds.codec.Encode(eds.rowSlice(i, 0, eds.originalDataWidth))
				if err != nil {
					return err
				}
				if !bytes.Equal(flattenChunks(parityChunks), flattenChunks(eds.rowSlice(i, eds.originalDataWidth, eds.originalDataWidth))) {
					return &ErrByzantineData{Row, i, eds.row(i)}
				}
				return nil
			})
		}

		colIsComplete := noMissingData(eds.col(i), noChunkInsertion)
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
				// check if we take the first half of the col and encode it, we get the second half
				parityChunks, err := eds.codec.Encode(eds.colSlice(0, i, eds.originalDataWidth))
				if err != nil {
					return err
				}
				if !bytes.Equal(flattenChunks(parityChunks), flattenChunks(eds.colSlice(eds.originalDataWidth, i, eds.originalDataWidth))) {
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

// computeChunksRoot calculates the root of the chunks for the specified axis (`i`th column or row).
func (eds *ExtendedDataSquare) computeChunksRoot(chunks [][]byte, axis Axis, i uint) ([]byte, error) {
	tree := eds.createTreeFn(axis, i)
	for _, d := range chunks {
		err := tree.Push(d)
		if err != nil {
			return nil, err
		}
	}
	return tree.Root()
}

// computeChunksRootWithRebuiltChunk computes the root of the chunks with the rebuilt chunk `rebuiltChunk` at the specified index `rebuiltIndex`.
func (eds *ExtendedDataSquare) computeChunksRootWithRebuiltChunk(chunks [][]byte, axis Axis, i uint, rebuiltIndex int, rebuiltChunk []byte) ([]byte, error) {
	tree := eds.createTreeFn(axis, i)
	for _, d := range chunks[:rebuiltIndex] {
		err := tree.Push(d)
		if err != nil {
			return nil, err
		}
	}

	err := tree.Push(rebuiltChunk)
	if err != nil {
		return nil, err
	}

	for _, d := range chunks[rebuiltIndex+1:] {
		err := tree.Push(d)
		if err != nil {
			return nil, err
		}
	}
	return tree.Root()
}
