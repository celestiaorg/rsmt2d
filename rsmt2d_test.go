//go:build leopard

package rsmt2d_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/celestiaorg/rsmt2d"
	"github.com/stretchr/testify/assert"
)

func FuzzRepairExtendedDataSquare(f *testing.F) {
	bufferSize := 64
	tests := []struct {
		name string
		// Size of each share, in bytes
		shareSize int
		codec     rsmt2d.Codec
	}{
		{"leopardFF8", bufferSize, rsmt2d.NewLeoRSFF8Codec()},
		{"leopardFF16", bufferSize, rsmt2d.NewLeoRSFF16Codec()},
		{"infectiousGF8", bufferSize, rsmt2d.NewRSGF8Codec()},
	}

	ones := bytes.Repeat([]byte{1}, bufferSize)
	twos := bytes.Repeat([]byte{2}, bufferSize)
	threes := bytes.Repeat([]byte{3}, bufferSize)
	fours := bytes.Repeat([]byte{4}, bufferSize)
	f.Add(ones, twos, threes, fours, []byte{0, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 13})

	f.Fuzz(func(t *testing.T, quadrant1, quadrant2, quadrant3, quadrant4, deletedShares []byte) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				shareSize := tt.shareSize
				// Compute parity shares
				eds, err := rsmt2d.ComputeExtendedDataSquare(
					[][]byte{
						padOrTrim(quadrant1, shareSize), padOrTrim(quadrant2, shareSize),
						padOrTrim(quadrant3, shareSize), padOrTrim(quadrant4, shareSize),
					},
					tt.codec,
					rsmt2d.NewDefaultTree,
				)
				if err != nil {
					t.Errorf("ComputeExtendedDataSquare failed: %v", err)
				}

				rowRoots := eds.RowRoots()
				colRoots := eds.ColRoots()

				// Save all shares in flattened form.
				flattened := make([][]byte, 0, eds.Width()*eds.Width())
				for i := uint(0); i < eds.Width(); i++ {
					flattened = append(flattened, eds.Row(i)...)
				}

				t.Run("RandomErasures", func(t *testing.T) {
					// Delete some shares, just enough so that repairing is always possible.
					deletions := padOrTrim(deletedShares, 7)
					for _, share := range deletions {
						flattened[int(share)%16] = nil
					}

					// Re-import the data square.
					eds, err = rsmt2d.ImportExtendedDataSquare(flattened, tt.codec, rsmt2d.NewDefaultTree)
					if err != nil {
						t.Errorf("ImportExtendedDataSquare failed: %v", err)
					}

					// Repair square.
					err = eds.Repair(
						rowRoots,
						colRoots,
					)
					if err != nil {
						// err contains information to construct a fraud proof
						// See extendeddatacrossword_test.go
						t.Errorf("RepairExtendedDataSquare failed: %v", err)
					} else {
						assert.Equal(t, eds.GetCell(0, 0), padOrTrim(quadrant1, shareSize))
						assert.Equal(t, eds.GetCell(0, 1), padOrTrim(quadrant2, shareSize))
						assert.Equal(t, eds.GetCell(1, 0), padOrTrim(quadrant3, shareSize))
						assert.Equal(t, eds.GetCell(1, 1), padOrTrim(quadrant4, shareSize))
					}
				})
				t.Run("MaximumErasures", func(t *testing.T) {
					// Delete some shares, just enough so that repairing is possible.
					// Repairing is only possible due to half of the shares being available in the last row.
					flattened[0], flattened[2], flattened[3] = nil, nil, nil
					flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
					flattened[8], flattened[9], flattened[10] = nil, nil, nil
					flattened[12], flattened[13] = nil, nil

					// Re-import the data square.
					eds, err = rsmt2d.ImportExtendedDataSquare(flattened, tt.codec, rsmt2d.NewDefaultTree)
					if err != nil {
						t.Errorf("ImportExtendedDataSquare failed: %v", err)
					}

					// Repair square.
					err = eds.Repair(
						rowRoots,
						colRoots,
					)
					if err != nil {
						// err contains information to construct a fraud proof
						// See extendeddatacrossword_test.go
						t.Errorf("RepairExtendedDataSquare failed: %v", err)
					} else {
						assert.Equal(t, eds.GetCell(0, 0), padOrTrim(quadrant1, shareSize))
						assert.Equal(t, eds.GetCell(0, 1), padOrTrim(quadrant2, shareSize))
						assert.Equal(t, eds.GetCell(1, 0), padOrTrim(quadrant3, shareSize))
						assert.Equal(t, eds.GetCell(1, 1), padOrTrim(quadrant4, shareSize))
					}
				})
				t.Run("TooManyErasures", func(t *testing.T) {
					// Delete some shares, just enough so that repairing is possible, then remove one more.
					flattened[0], flattened[1], flattened[2], flattened[3] = nil, nil, nil, nil
					flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
					flattened[8], flattened[9], flattened[10] = nil, nil, nil
					flattened[12], flattened[13] = nil, nil

					// Re-import the data square.
					eds, err = rsmt2d.ImportExtendedDataSquare(flattened, tt.codec, rsmt2d.NewDefaultTree)
					if err != nil {
						t.Errorf("ImportExtendedDataSquare failed: %v", err)
					}

					// Repair square.
					err = eds.Repair(
						rowRoots,
						colRoots,
					)
					if !errors.Is(err, rsmt2d.ErrUnrepairableDataSquare) {
						// Should fail since insufficient data.
						t.Errorf("RepairExtendedDataSquare did not fail with `%v`, got `%v`", rsmt2d.ErrUnrepairableDataSquare, err)
					}
				})
			})
		}
	})
}

func padOrTrim(input []byte, size int) []byte {
	length := len(input)
	if length < size {
		share := make([]byte, size)
		copy(share[size-length:], input)
		return share
	} else if length > size {
		return input[length-size:]
	} else {
		return input
	}
}
