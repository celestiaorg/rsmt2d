package rsmt2d

import (
	"bytes"
	"crypto/md5"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepairExtendedDataSquare(t *testing.T) {
	for _, codec := range codecs {
		codec := codec.codecType()

		bufferSize := 64
		original, err := ComputeExtendedDataSquare([][]byte{
			makeCheckedRandBytes(bufferSize), makeCheckedRandBytes(bufferSize),
			makeCheckedRandBytes(bufferSize), makeCheckedRandBytes(bufferSize),
		}, codec)
		if err != nil {
			panic(err)
		}

		flattened := original.flattened()
		flattened[0], flattened[2], flattened[3] = nil, nil, nil
		flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
		flattened[8], flattened[9], flattened[10] = nil, nil, nil
		flattened[12], flattened[13] = nil, nil
		var result *ExtendedDataSquare
		result, err = RepairExtendedDataSquare(original.RowRoots(), original.ColumnRoots(), flattened, codec)
		if err != nil {
			t.Fatalf("unexpected err while repairing data square: %v, codec: :%v", err, codec)
		}
		//if !reflect.DeepEqual(result.square, [][][]byte{
		//	{{1}, {2}, {7}, {13}},
		//	{{3}, {4}, {13}, {31}},
		//	{{5}, {14}, {19}, {41}},
		//	{{9}, {26}, {47}, {69}},
		//}) {
		//	t.Errorf("failed to repair a repairable square")
		//}
		assert.Equal(t, true, checkBytes(result.square[0][0]))
		assert.Equal(t, true, checkBytes(result.square[0][1]))
		assert.Equal(t, true, checkBytes(result.square[1][0]))
		assert.Equal(t, true, checkBytes(result.square[1][1]))

		flattened = original.flattened()
		flattened[0], flattened[2], flattened[3] = nil, nil, nil
		flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
		flattened[8], flattened[9], flattened[10] = nil, nil, nil
		flattened[12], flattened[13], flattened[14] = nil, nil, nil
		_, err = RepairExtendedDataSquare(original.RowRoots(), original.ColumnRoots(), flattened, codec)
		if err == nil {
			t.Errorf("did not return an error on trying to repair an unrepairable square")
		}
		// TODO: figure out why these tests fail too now (only increased bufferBytes from 1 to 64):
		// They probably just assume the old one-byte chunk layout ...
		var corrupted ExtendedDataSquare
		corrupted, err = original.deepCopy()
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%v", err, codec)
		}
		corrupted.setCell(0, 0, []byte{66})
		_, err = RepairExtendedDataSquare(original.RowRoots(), original.ColumnRoots(), corrupted.flattened(), codec)
		if err == nil {
			t.Errorf("did not return an error on trying to repair a square with bad roots")
		}

		var ok bool
		corrupted, err = original.deepCopy()
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%v", err, codec)
		}
		corrupted.setCell(0, 0, []byte{66})
		_, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), corrupted.flattened(), codec)
		if err, ok = err.(*ByzantineRowError); !ok {
			t.Errorf("did not return a ByzantineRowError for a bad row; got: %v", err)
		}

		corrupted, err = original.deepCopy()
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%v", err, codec)
		}
		corrupted.setCell(0, 3, []byte{66})
		_, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), corrupted.flattened(), codec)
		if err, ok = err.(*ByzantineRowError); !ok {
			t.Errorf("did not return a ByzantineRowError for a bad row; got %v", err)
		}

		corrupted, err = original.deepCopy()
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%v", err, codec)
		}
		corrupted.setCell(0, 0, []byte{66})
		flattened = corrupted.flattened()
		flattened[1], flattened[2], flattened[3] = nil, nil, nil
		_, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), flattened, codec)
		if err, ok = err.(*ByzantineColumnError); !ok {
			t.Errorf("did not return a ByzantineColumnError for a bad column; got %v", err)
		}

		corrupted, err = original.deepCopy()
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%v", err, codec)
		}
		corrupted.setCell(3, 0, []byte{66})
		flattened = corrupted.flattened()
		flattened[1], flattened[2], flattened[3] = nil, nil, nil
		_, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), flattened, codec)
		if err, ok = err.(*ByzantineColumnError); !ok {
			t.Errorf("did not return a ByzantineColumnError for a bad column; got %v", err)
		}
	}
}

func makeCheckedRandBytes(num int) []byte {
	p := make([]byte, num)
	if len(p) <= md5.Size {
		panic("provided slice is too small")
	}
	raw := make([]byte, len(p)-md5.Size)
	rand.Read(raw)
	chksm := md5.Sum(raw)
	copy(p, raw)
	copy(p[len(p)-md5.Size:], chksm[:])
	return p
}

func checkBytes(p []byte) bool {
	if len(p) <= md5.Size {
		panic("provided slice is too small")
	}
	data := p[:len(p)-md5.Size]
	readChksm := p[len(p)-md5.Size:]
	chksm := md5.Sum(data)
	if !bytes.Equal(readChksm, chksm[:]) {
		return false
	}
	return true
}
