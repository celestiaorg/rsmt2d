package rsmt2d

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PseudoFraudProof is an example fraud proof.
// TODO a real fraud proof would have a Merkle proof for each share.
type PseudoFraudProof struct {
	Mode   int      // Row (0) or column (1)
	Index  uint     // Row or column index
	Shares [][]byte // Bad shares (nil are missing)
}

func TestRepairExtendedDataSquare(t *testing.T) {
	bufferSize := 64
	tests := []struct {
		name string
		// Size of each share, in bytes
		shareSize int
		codec     Codec
	}{
		{"leopard", bufferSize, NewLeoRSCodec()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			name, codec, shareSize := test.name, test.codec, test.shareSize
			original := createTestEds(codec, shareSize)

			rowRoots, err := original.RowRoots()
			require.NoError(t, err)

			colRoots, err := original.ColRoots()
			require.NoError(t, err)

			// Verify that an EDS can be repaired after the maximum amount of erasures
			t.Run("MaximumErasures", func(t *testing.T) {
				flattened := original.Flattened()
				flattened[0], flattened[2], flattened[3] = nil, nil, nil
				flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
				flattened[8], flattened[9], flattened[10] = nil, nil, nil
				flattened[12], flattened[13] = nil, nil

				// Re-import the data square.
				eds, err := ImportExtendedDataSquare(flattened, codec, NewDefaultTree)
				if err != nil {
					t.Errorf("ImportExtendedDataSquare failed: %v", err)
				}

				err = eds.Repair(rowRoots, colRoots)
				if err != nil {
					t.Errorf("unexpected err while repairing data square: %v, codec: :%s", err, name)
				} else {
					assert.Equal(t, original.GetCell(0, 0), bytes.Repeat([]byte{1}, shareSize))
					assert.Equal(t, original.GetCell(0, 1), bytes.Repeat([]byte{2}, shareSize))
					assert.Equal(t, original.GetCell(1, 0), bytes.Repeat([]byte{3}, shareSize))
					assert.Equal(t, original.GetCell(1, 1), bytes.Repeat([]byte{4}, shareSize))
				}
			})

			// Verify that an EDS returns an error when there are too many erasures
			t.Run("Unrepairable", func(t *testing.T) {
				flattened := original.Flattened()
				flattened[0], flattened[2], flattened[3] = nil, nil, nil
				flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
				flattened[8], flattened[9], flattened[10] = nil, nil, nil
				flattened[12], flattened[13], flattened[14] = nil, nil, nil

				// Re-import the data square.
				eds, err := ImportExtendedDataSquare(flattened, codec, NewDefaultTree)
				if err != nil {
					t.Errorf("ImportExtendedDataSquare failed: %v", err)
				}

				err = eds.Repair(rowRoots, colRoots)
				if err != ErrUnrepairableDataSquare {
					t.Errorf("did not return an error on trying to repair an unrepairable square")
				}
			})
		})
	}
}

func TestValidFraudProof(t *testing.T) {
	bufferSize := 64
	corruptChunk := bytes.Repeat([]byte{66}, bufferSize)
	tests := []struct {
		name string
		// Size of each share, in bytes
		shareSize int
		codec     Codec
	}{
		{"leopard", bufferSize, NewLeoRSCodec()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			name, codec, shareSize := test.name, test.codec, test.shareSize
			original := createTestEds(codec, shareSize)

			var byzData *ErrByzantineData
			corrupted, err := original.deepCopy(codec)
			if err != nil {
				t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, name)
			}
			corrupted.setCell(0, 0, corruptChunk)
			assert.NoError(t, err)

			rowRoots, err := corrupted.getRowRoots()
			assert.NoError(t, err)

			colRoots, err := corrupted.getColRoots()
			assert.NoError(t, err)

			err = corrupted.Repair(rowRoots, colRoots)
			errors.As(err, &byzData)

			// Construct the fraud proof
			fraudProof := PseudoFraudProof{0, byzData.Index, byzData.Shares}
			// Verify the fraud proof
			// TODO in a real fraud proof, also verify Merkle proof for each non-nil share.
			rebuiltShares, err := codec.Decode(fraudProof.Shares)
			if err != nil {
				t.Errorf("could not decode fraud proof shares; got: %v", err)
			}
			root, err := corrupted.computeSharesRoot(rebuiltShares, byzData.Axis, fraudProof.Index)
			assert.NoError(t, err)
			rowRoot, err := corrupted.getRowRoot(fraudProof.Index)
			assert.NoError(t, err)
			if bytes.Equal(root, rowRoot) {
				// If the roots match, then the fraud proof should be for invalid erasure coding.
				parityShares, err := codec.Encode(rebuiltShares[0:corrupted.originalDataWidth])
				if err != nil {
					t.Errorf("could not encode fraud proof shares; %v", fraudProof)
				}
				startIndex := len(rebuiltShares) - int(corrupted.originalDataWidth)
				if bytes.Equal(flattenChunks(parityShares), flattenChunks(rebuiltShares[startIndex:])) {
					t.Errorf("invalid fraud proof %v", fraudProof)
				}
			}
		})
	}
}

func TestCannotRepairSquareWithBadRoots(t *testing.T) {
	bufferSize := 64
	corruptChunk := bytes.Repeat([]byte{66}, bufferSize)
	tests := []struct {
		name string
		// Size of each share, in bytes
		shareSize int
		codec     Codec
	}{
		{"leopard", bufferSize, NewLeoRSCodec()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			codec, shareSize := test.codec, test.shareSize
			original := createTestEds(codec, shareSize)

			rowRoots, err := original.RowRoots()
			require.NoError(t, err)

			colRoots, err := original.ColRoots()
			require.NoError(t, err)

			original.setCell(0, 0, corruptChunk)
			require.NoError(t, err)
			err = original.Repair(rowRoots, colRoots)
			if err == nil {
				t.Errorf("did not return an error on trying to repair a square with bad roots")
			}
		})
	}
}

func TestCorruptedEdsReturnsErrByzantineData(t *testing.T) {
	shareSize := 64
	corruptChunk := bytes.Repeat([]byte{66}, shareSize)

	tests := []struct {
		name   string
		coords [][]uint
		values [][]byte
	}{
		{
			name:   "corrupt a chunk in the original data square",
			coords: [][]uint{{0, 0}},
			values: [][]byte{corruptChunk},
		},
		{
			name:   "corrupt a chunk in the extended data square",
			coords: [][]uint{{0, 3}},
			values: [][]byte{corruptChunk},
		},
		{
			name:   "corrupt a chunk at (0, 0) and delete shares from the rest of the row",
			coords: [][]uint{{0, 0}, {0, 1}, {0, 2}, {0, 3}},
			values: [][]byte{corruptChunk, nil, nil, nil},
		},
		{
			name:   "corrupt a chunk at (3, 0) and delete part of the first row ",
			coords: [][]uint{{3, 0}, {0, 1}, {0, 2}, {0, 3}},
			values: [][]byte{corruptChunk, nil, nil, nil},
		},
		{
			// This test case sets all shares along the diagonal to nil so that
			// the prerepairSanityCheck does not return an error and it can
			// verify that solveCrossword returns an ErrByzantineData with
			// shares populated.
			name: "set all shares along the diagonal to nil and then corrupt the cell at (0, 1)",
			// In the ASCII diagram below, _ represents a nil share and C
			// represents a corrupted share.
			//
			// _ C O O
			// O _ O O
			// O O _ O
			// O O O _
			coords: [][]uint{{0, 0}, {1, 1}, {2, 2}, {3, 3}, {0, 1}},
			values: [][]byte{nil, nil, nil, nil, corruptChunk},
		},
	}

	for codecName, codec := range codecs {
		t.Run(codecName, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					eds := createTestEds(codec, shareSize)
					for i, coords := range test.coords {
						x := coords[0]
						y := coords[1]
						eds.setCell(x, y, test.values[i])
					}
					rowRoots, err := eds.getRowRoots()
					assert.NoError(t, err)

					colRoots, err := eds.getColRoots()
					assert.NoError(t, err)

					err = eds.Repair(rowRoots, colRoots)
					assert.Error(t, err)

					// due to parallelisation, the ErrByzantineData axis may be either row or col
					var byzData *ErrByzantineData
					assert.ErrorAs(t, err, &byzData, "did not return a ErrByzantineData for a bad col or row")
					assert.NotEmpty(t, byzData.Shares)
					assert.Contains(t, byzData.Shares, corruptChunk)
				})
			}
		})
	}
}

func BenchmarkRepair(b *testing.B) {
	chunkSize := uint(256)
	// For different ODS sizes
	for originalDataWidth := 4; originalDataWidth <= 512; originalDataWidth *= 2 {
		for codecName, codec := range codecs {
			if codec.MaxChunks() < originalDataWidth*originalDataWidth {
				// Only test codecs that support this many chunks
				continue
			}

			// Generate a new range original data square then extend it
			square := genRandDS(originalDataWidth, int(chunkSize))
			eds, err := ComputeExtendedDataSquare(square, codec, NewDefaultTree)
			if err != nil {
				b.Error(err)
			}

			extendedDataWidth := originalDataWidth * 2
			rowRoots, err := eds.RowRoots()
			assert.NoError(b, err)

			colRoots, err := eds.ColRoots()
			assert.NoError(b, err)

			b.Run(
				fmt.Sprintf(
					"%s %dx%dx%d ODS",
					codecName,
					originalDataWidth,
					originalDataWidth,
					len(square[0]),
				),
				func(b *testing.B) {
					for n := 0; n < b.N; n++ {
						b.StopTimer()

						flattened := eds.Flattened()
						// Randomly remove 1/2 of the shares of each row
						for r := 0; r < extendedDataWidth; r++ {
							for c := 0; c < originalDataWidth; {
								ind := rand.Intn(extendedDataWidth)
								if flattened[r*extendedDataWidth+ind] == nil {
									continue
								}
								flattened[r*extendedDataWidth+ind] = nil
								c++
							}
						}

						// Re-import the data square.
						eds, _ = ImportExtendedDataSquare(flattened, codec, NewDefaultTree)

						b.StartTimer()

						err := eds.Repair(
							rowRoots,
							colRoots,
						)
						if err != nil {
							b.Error(err)
						}
					}
				},
			)
		}
	}
}

func createTestEds(codec Codec, shareSize int) *ExtendedDataSquare {
	ones := bytes.Repeat([]byte{1}, shareSize)
	twos := bytes.Repeat([]byte{2}, shareSize)
	threes := bytes.Repeat([]byte{3}, shareSize)
	fours := bytes.Repeat([]byte{4}, shareSize)

	eds, err := ComputeExtendedDataSquare([][]byte{
		ones, twos,
		threes, fours,
	}, codec, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	return eds
}
