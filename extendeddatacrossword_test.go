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
	for _, codec := range codecs {
		codec := codec.codecType()

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

		flattened := original.flattened()
		flattened[0], flattened[2], flattened[3] = nil, nil, nil
		flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
		flattened[8], flattened[9], flattened[10] = nil, nil, nil
		flattened[12], flattened[13] = nil, nil
		var result *ExtendedDataSquare
		result, err = RepairExtendedDataSquare(original.RowRoots(), original.ColumnRoots(), flattened, codec, NewDefaultTree)
		if err != nil {
			t.Errorf("unexpected err while repairing data square: %v, codec: :%v", err, codec)
		} else {
			assert.Equal(t, result.square[0][0], ones)
			assert.Equal(t, result.square[0][1], twos)
			assert.Equal(t, result.square[1][0], threes)
			assert.Equal(t, result.square[1][1], fours)
		}

		flattened = original.flattened()
		flattened[0], flattened[2], flattened[3] = nil, nil, nil
		flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
		flattened[8], flattened[9], flattened[10] = nil, nil, nil
		flattened[12], flattened[13], flattened[14] = nil, nil, nil
		_, err = RepairExtendedDataSquare(original.RowRoots(), original.ColumnRoots(), flattened, codec, NewDefaultTree)
		if err == nil {
			t.Errorf("did not return an error on trying to repair an unrepairable square")
		}
		var corrupted ExtendedDataSquare
		corrupted, err = original.deepCopy()
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%v", err, codec)
		}
		corruptChunk := bytes.Repeat([]byte{66}, bufferSize)
		corrupted.setCell(0, 0, corruptChunk)
		_, err = RepairExtendedDataSquare(original.RowRoots(), original.ColumnRoots(), corrupted.flattened(), codec, NewDefaultTree)
		if err == nil {
			t.Errorf("did not return an error on trying to repair a square with bad roots")
		}

		corrupted, err = original.deepCopy()
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%v", err, codec)
		}
		corrupted.setCell(0, 0, corruptChunk)
		_, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), corrupted.flattened(), codec, NewDefaultTree)
		var byzRow *ErrByzantineRow
		if !errors.As(err, &byzRow) {
			t.Errorf("did not return a ErrByzantineRow for a bad row; got: %v", err)
		}

		// Construct the fraud proof
		fraudProof := PseudoFraudProof{row, byzRow.RowNumber, byzRow.Shares}
		// Verify the fraud proof
		// TODO in a real fraud proof, also verify Merkle proof for each non-nil share.
		rebuiltShares, err := Decode(fraudProof.Shares, codec)
		if err != nil {
			t.Errorf("could not decode fraud proof shares; got: %v", err)
		}
		root := corrupted.computeSharesRoot(rebuiltShares, fraudProof.Index)
		if bytes.Equal(root, corrupted.RowRoot(fraudProof.Index)) {
			// If the roots match, then the fraud proof should be for invalid erasure coding.
			parityShares, err := Encode(rebuiltShares[0:corrupted.originalDataWidth], codec)
			if err != nil {
				t.Errorf("could not encode fraud proof shares; %v", fraudProof)
			}
			startIndex := len(rebuiltShares) - int(corrupted.originalDataWidth)
			if bytes.Equal(flattenChunks(parityShares), flattenChunks(rebuiltShares[startIndex:])) {
				t.Errorf("invalid fraud proof %v", fraudProof)
			}
		}

		corrupted, err = original.deepCopy()
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%v", err, codec)
		}
		corrupted.setCell(0, 3, corruptChunk)
		_, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), corrupted.flattened(), codec, NewDefaultTree)
		if !errors.As(err, &byzRow) {
			t.Errorf("did not return a ErrByzantineRow for a bad row; got %v", err)
		}

		corrupted, err = original.deepCopy()
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%v", err, codec)
		}
		corrupted.setCell(0, 0, corruptChunk)
		flattened = corrupted.flattened()
		flattened[1], flattened[2], flattened[3] = nil, nil, nil
		_, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), flattened, codec, NewDefaultTree)
		var byzColumn *ErrByzantineColumn
		if !errors.As(err, &byzColumn) {
			t.Errorf("did not return a ErrByzantineColumn for a bad column; got %v", err)
		}

		corrupted, err = original.deepCopy()
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%v", err, codec)
		}
		corrupted.setCell(3, 0, corruptChunk)
		flattened = corrupted.flattened()
		flattened[1], flattened[2], flattened[3] = nil, nil, nil
		_, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), flattened, codec, NewDefaultTree)
		if !errors.As(err, &byzColumn) {
			t.Errorf("did not return a ErrByzantineColumn for a bad column; got %v", err)
		}
	}
}

func BenchmarkRepair(b *testing.B) {
	// For different ODS sizes
	for i := 16; i <= 128; i *= 2 {
		for _codecType := range codecs {
			// Generate a new range original data square then extend it
			square := genRandDS(i)
			eds, err := ComputeExtendedDataSquare(square, _codecType, NewDefaultTree)
			if err != nil {
				b.Error(err)
			}

			flattened := eds.flattened()
			// Randomly remove 1/2 of the shares of each row
			for r := 0; r < i*2; r++ {
				for c := 0; c < i; {
					ind := rand.Intn(i + 1)
					if flattened[r*i+ind] == nil {
						continue
					}
					flattened[r*i+ind] = nil
					c++
				}
			}

			b.Run(
				fmt.Sprintf("Repairing %dx%d ODS using %s", i, i, _codecType),
				func(b *testing.B) {
					for n := 0; n < b.N; n++ {
						_, err := RepairExtendedDataSquare(
							eds.RowRoots(),
							eds.ColumnRoots(),
							flattened,
							_codecType,
							NewDefaultTree,
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
