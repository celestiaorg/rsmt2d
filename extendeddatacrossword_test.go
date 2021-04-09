package rsmt2d

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			// Randomly remove half the shares
			for i := 0; i < i*i*2; {
				ind := rand.Intn(i)
				if len(flattened[ind]) == 0 {
					continue
				}
				flattened[ind] = []byte{}
				i++
			}

			b.Run(
				fmt.Sprintf("Repairing %dx%d ODS using %s", i, i, _codecType),
				func(b *testing.B) {
					for n := 0; n < b.N; n++ {
						_, err := RepairExtendedDataSquare(eds.RowRoots(),
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
