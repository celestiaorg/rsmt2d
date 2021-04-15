package rsmt2d

import (
	"bytes"
	"errors"
	"fmt"
	"math"
)

const (
	row = iota
	column
)

// ErrUnrepairableDataSquare is thrown when there is insufficient chunks to repair the square.
var ErrUnrepairableDataSquare = errors.New("failed to solve data square")

// ErrByzantineRow is thrown when there is a repaired row does not match the expected row Merkle root.
type ErrByzantineRow struct {
	RowNumber uint     // Row index
	Shares    [][]byte // Pre-repaired row shares
}

func (e *ErrByzantineRow) Error() string {
	return fmt.Sprintf("byzantine row: %d", e.RowNumber)
}

// ErrByzantineColumn is thrown when there is a repaired column does not match the expected column Merkle root.
type ErrByzantineColumn struct {
	ColumnNumber uint     // Column index
	Shares       [][]byte // Pre-repaired column shares
}

func (e *ErrByzantineColumn) Error() string {
	return fmt.Sprintf("byzantine column: %d", e.ColumnNumber)
}

// RepairExtendedDataSquare attempts to repair an incomplete extended data
// square (EDS), comparing repaired rows and columns against expected Merkle
// roots.
//
// Input
//
// Missing data chunks should be represented as nil.
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
	columnRoots [][]byte,
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

	err = eds.prerepairSanityCheck(rowRoots, columnRoots, bitMat, codec)
	if err != nil {
		return nil, err
	}

	err = eds.solveCrossword(rowRoots, columnRoots, bitMat, codec)
	if err != nil {
		return nil, err
	}

	return eds, err
}

func (eds *ExtendedDataSquare) solveCrossword(rowRoots [][]byte, columnRoots [][]byte, bitMask bitMatrix, codec Codec) error {
	// Keep repeating until the square is solved
	solved := false
	for {
		solved = true
		progressMade := false

		// Loop through every row and column, attempt to rebuild each row or column if incomplete
		for i := 0; i < int(eds.width); i++ {
			for mode := range []int{row, column} {
				var isIncomplete bool
				var isExtendedPartIncomplete bool
				switch mode {
				case row:
					isIncomplete = !bitMask.RowIsOne(i)
					isExtendedPartIncomplete = !bitMask.RowRangeIsOne(i, int(eds.originalDataWidth), int(eds.width))
				case column:
					isIncomplete = !bitMask.ColumnIsOne(i)
					isExtendedPartIncomplete = !bitMask.ColRangeIsOne(i, int(eds.originalDataWidth), int(eds.width))
				default:
					panic(fmt.Sprintf("invalid mode %d", mode))
				}

				if isIncomplete { // row/column incomplete
					// Prepare shares
					shares := make([][]byte, eds.width)
					for j := 0; j < int(eds.width); j++ {
						var vectorData [][]byte
						var r, c int
						switch mode {
						case row:
							r = i
							c = j
							vectorData = eds.Row(uint(i))
						case column:
							r = j
							c = i
							vectorData = eds.Column(uint(i))
						default:
							panic(fmt.Sprintf("invalid mode %d", mode))
						}
						if bitMask.Get(r, c) {
							// As guaranteed by the bitMask, vectorData can't be nil here:
							shares[j] = vectorData[j]
						}
					}

					// Attempt rebuild
					rebuiltShares, err := codec.Decode(shares)
					if err != nil {
						// repair unsuccessful
						solved = false
						continue
					}

					progressMade = true

					if isExtendedPartIncomplete {
						// If needed, rebuild the parity shares too.
						rebuiltExtendedShares, err := codec.Encode(rebuiltShares[0:eds.originalDataWidth])
						if err != nil {
							return err
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

					// Check that rebuilt shares matches appropriate root
					err = eds.verifyAgainstRoots(rowRoots, columnRoots, mode, uint(i), bitMask, rebuiltShares)
					if err != nil {
						return err
					}

					// Check that newly completed orthogonal vectors match their new merkle roots
					for j := 0; j < int(eds.width); j++ {
						switch mode {
						case row:
							if !bitMask.Get(i, j) &&
								bitMask.ColumnIsOne(j) {
								err := eds.verifyAgainstRoots(rowRoots, columnRoots, column, uint(j), bitMask, rebuiltShares)
								if err != nil {
									return err
								}
							}

						case column:
							if !bitMask.Get(j, i) &&
								bitMask.RowIsOne(j) {
								err := eds.verifyAgainstRoots(rowRoots, columnRoots, row, uint(j), bitMask, rebuiltShares)
								if err != nil {
									return err
								}
							}

						default:
							panic(fmt.Sprintf("invalid mode %d", mode))
						}
					}

					// Set vector mask to true
					switch mode {
					case row:
						for j := 0; j < int(eds.width); j++ {
							bitMask.Set(i, j)
						}
					case column:
						for j := 0; j < int(eds.width); j++ {
							bitMask.Set(j, i)
						}
					default:
						panic(fmt.Sprintf("invalid mode %d", mode))
					}

					// Insert rebuilt shares into square.
					for p, s := range rebuiltShares {
						switch mode {
						case row:
							eds.setCell(uint(i), uint(p), s)
						case column:
							eds.setCell(uint(p), uint(i), s)
						default:
							panic(fmt.Sprintf("invalid mode %d", mode))
						}
					}
				}
			}
		}

		if solved {
			break
		} else if !progressMade {
			return ErrUnrepairableDataSquare
		}
	}

	return nil
}

func (eds *ExtendedDataSquare) verifyAgainstRoots(rowRoots [][]byte, columnRoots [][]byte, mode int, i uint, bitMask bitMatrix, shares [][]byte) error {
	root := eds.computeSharesRoot(shares, i)

	switch mode {
	case row:
		if !bytes.Equal(root, rowRoots[i]) {
			for c := 0; c < int(eds.width); c++ {
				if !bitMask.Get(int(i), c) {
					shares[c] = nil
				}
			}
			return &ErrByzantineRow{i, shares}
		}
	case column:
		if !bytes.Equal(root, columnRoots[i]) {
			for r := 0; r < int(eds.width); r++ {
				if !bitMask.Get(r, int(i)) {
					shares[r] = nil
				}
			}
			return &ErrByzantineColumn{i, shares}
		}
	default:
		panic(fmt.Sprintf("invalid mode %d", mode))
	}
	return nil
}

func (eds *ExtendedDataSquare) prerepairSanityCheck(rowRoots [][]byte, columnRoots [][]byte, bitMask bitMatrix, codec Codec) error {
	for i := uint(0); i < eds.width; i++ {
		rowIsComplete := bitMask.RowIsOne(int(i))
		colIsComplete := bitMask.ColumnIsOne(int(i))

		// if there's no missing data in the this row
		if noMissingData(eds.Row(i)) {
			// ensure that the roots are equal and that rowMask is a vector
			if rowIsComplete && !bytes.Equal(rowRoots[i], eds.RowRoot(i)) {
				return fmt.Errorf("bad root input: row %d expected %v got %v", i, rowRoots[i], eds.RowRoot(i))
			}
		}

		// if there's no missing data in the this col
		if noMissingData(eds.Column(i)) {
			// ensure that the roots are equal and that rowMask is a vector
			if colIsComplete && !bytes.Equal(columnRoots[i], eds.ColRoot(i)) {
				return fmt.Errorf("bad root input: col %d expected %v got %v", i, columnRoots[i], eds.ColRoot(i))
			}
		}

		if rowIsComplete {
			parityShares, err := codec.Encode(eds.rowSlice(i, 0, eds.originalDataWidth))
			if err != nil {
				return err
			}
			if !bytes.Equal(flattenChunks(parityShares), flattenChunks(eds.rowSlice(i, eds.originalDataWidth, eds.originalDataWidth))) {
				return &ErrByzantineRow{i, eds.Row(i)}
			}
		}

		if colIsComplete {
			parityShares, err := codec.Encode(eds.columnSlice(0, i, eds.originalDataWidth))
			if err != nil {
				return err
			}
			if !bytes.Equal(flattenChunks(parityShares), flattenChunks(eds.columnSlice(eds.originalDataWidth, i, eds.originalDataWidth))) {
				return &ErrByzantineColumn{i, eds.Column(i)}
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
