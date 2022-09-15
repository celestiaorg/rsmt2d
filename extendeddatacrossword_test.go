package rsmt2d

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

// PseudoFraudProof is an example fraud proof.
// TODO a real fraud proof would have a Merkle proof for each share.
type PseudoFraudProof struct {
	Mode   int      // Row (0) or column (1)
	Index  uint     // Row or column index
	Shares [][]byte // Bad shares (nil are missing)
}

func TestRepairExtendedDataSquare(t *testing.T) {
	for codecName, codec := range codecs {

		bufferSize := 64
		ones := bytes.Repeat([]byte{1}, bufferSize)
		twos := bytes.Repeat([]byte{2}, bufferSize)
		threes := bytes.Repeat([]byte{3}, bufferSize)
		fours := bytes.Repeat([]byte{4}, bufferSize)

		original, err := ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, codec, NewDefaultTree)
		if err != nil {
			panic(err)
		}

		rowRoots := original.RowRoots()
		colRoots := original.ColRoots()

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
			t.Errorf("unexpected err while repairing data square: %v, codec: :%s", err, codecName)
		} else {
			assert.Equal(t, original.GetCell(0, 0), ones)
			assert.Equal(t, original.GetCell(0, 1), twos)
			assert.Equal(t, original.GetCell(1, 0), threes)
			assert.Equal(t, original.GetCell(1, 1), fours)
		}

		flattened = original.Flattened()
		flattened[0], flattened[2], flattened[3] = nil, nil, nil
		flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
		flattened[8], flattened[9], flattened[10] = nil, nil, nil
		flattened[12], flattened[13], flattened[14] = nil, nil, nil

		// Re-import the data square.
		eds, err = ImportExtendedDataSquare(flattened, codec, NewDefaultTree)
		if err != nil {
			t.Errorf("ImportExtendedDataSquare failed: %v", err)
		}

		err = eds.Repair(rowRoots, colRoots)
		if err == nil {
			t.Errorf("did not return an error on trying to repair an unrepairable square")
		}
		var corrupted ExtendedDataSquare
		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corruptChunk := bytes.Repeat([]byte{66}, bufferSize)
		corrupted.setCell(0, 0, corruptChunk)
		err = corrupted.Repair(rowRoots, colRoots)
		if err == nil {
			t.Errorf("did not return an error on trying to repair a square with bad roots")
		}

		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corrupted.setCell(0, 0, corruptChunk)
		err = corrupted.Repair(corrupted.getRowRoots(), corrupted.getColRoots())
		var byzData *ErrByzantineData
		if !errors.As(err, &byzData) || byzData.Axis != Row {
			t.Errorf("did not return a ErrByzantineData for a bad row; got: %v", err)
		}
		// Construct the fraud proof
		fraudProof := PseudoFraudProof{0, byzData.Index, byzData.Shares}
		// Verify the fraud proof
		// TODO in a real fraud proof, also verify Merkle proof for each non-nil share.
		rebuiltShares, err := codec.Decode(fraudProof.Shares)
		if err != nil {
			t.Errorf("could not decode fraud proof shares; got: %v", err)
		}
		root := corrupted.computeSharesRoot(rebuiltShares, byzData.Axis, fraudProof.Index)
		if bytes.Equal(root, corrupted.getRowRoot(fraudProof.Index)) {
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

		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corrupted.setCell(0, 3, corruptChunk)
		err = corrupted.Repair(corrupted.getRowRoots(), corrupted.getColRoots())
		if !errors.As(err, &byzData) || byzData.Axis != Row {
			t.Errorf("did not return a ErrByzantineData for a bad row; got %v", err)
		}

		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corrupted.setCell(0, 0, corruptChunk)
		corrupted.setCell(0, 1, nil)
		corrupted.setCell(0, 2, nil)
		corrupted.setCell(0, 3, nil)
		err = corrupted.Repair(corrupted.getRowRoots(), corrupted.getColRoots())
		if !errors.As(err, &byzData) || byzData.Axis != Col {
			t.Errorf("did not return a ErrByzantineData for a bad column; got %v", err)
		}
		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corrupted.setCell(3, 0, corruptChunk)
		corrupted.setCell(0, 1, nil)
		corrupted.setCell(0, 2, nil)
		corrupted.setCell(0, 3, nil)
		err = corrupted.Repair(corrupted.getRowRoots(), corrupted.getColRoots())
		if !errors.As(err, &byzData) || byzData.Axis != Col {
			t.Errorf("did not return a ErrByzantineData for a bad column; got %v", err)
		}
	}
}

func BenchmarkRepair(b *testing.B) {
	// For different ODS sizes
	for originalDataWidth := 16; originalDataWidth <= 128; originalDataWidth *= 2 {
		for codecName, codec := range codecs {
			// Generate a new range original data square then extend it
			square := genRandDS(originalDataWidth)
			eds, err := ComputeExtendedDataSquare(square, codec, NewDefaultTree)
			if err != nil {
				b.Error(err)
			}

			extendedDataWidth := originalDataWidth * 2
			rowRoots := eds.RowRoots()
			colRoots := eds.ColRoots()

			b.Run(
				fmt.Sprintf(
					"Repairing %dx%d ODS using %s",
					originalDataWidth,
					originalDataWidth,
					codecName,
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
