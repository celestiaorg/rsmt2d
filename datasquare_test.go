package rsmt2d

import (
	"reflect"
	"testing"
)

func TestNewDataSquare(t *testing.T) {
	result, err := newDataSquare([][]byte{{1, 2}})
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(result.square, [][][]byte{{{1, 2}}}) {
		t.Errorf("newDataSquare failed for 1x1 square")
	}

	result, err = newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}})
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(result.square, [][][]byte{{{1, 2}, {3, 4}}, {{5, 6}, {7, 8}}}) {
		t.Errorf("newDataSquare failed for 2x2 square")
	}

	result, err = newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}})
	if err == nil {
		t.Errorf("newDataSquare failed; inconsistent number of chunks accepted")
	}

	result, err = newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7}})
	if err == nil {
		t.Errorf("newDataSquare failed; chunks of unequal size accepted")
	}
}

func TestExtendSquare(t *testing.T) {
	ds, err := newDataSquare([][]byte{{1, 2}})
	if err != nil {
		panic(err)
	}
	err = ds.extendSquare(1, []byte{0})
	if err == nil {
		t.Errorf("extendSquare failed; error not returned when filler chunk size does not match data square chunk size")
	}

	ds, err = newDataSquare([][]byte{{1, 2}})
	if err != nil {
		panic(err)
	}
	err = ds.extendSquare(1, []byte{0, 0})
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(ds.square, [][][]byte{{{1, 2}, {0, 0}}, {{0, 0}, {0, 0}}}) {
		t.Errorf("extendSquare failed; unexpected result when extending 1x1 square to 2x2 square")
	}
}

func TestRoots(t *testing.T) {
	result, err := newDataSquare([][]byte{{1, 2}})
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(result.RowRoots(), result.ColumnRoots()) {
		t.Errorf("computing roots failed; expecting row and column roots for 1x1 square to be equal")
	}
}

func TestProofs(t *testing.T) {
	result, err := newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}})
	if err != nil {
		panic(err)
	}
	_, proof, proofIndex, numLeaves := result.computeRowProof(1, 1)
	if len(proof) != 2 {
		t.Errorf("computing row proof for (1, 1) in 2x2 square failed; expecting proof set of length 2")
	}
	if proofIndex != 1 {
		t.Errorf("computing row proof for (1, 1) in 2x2 square failed; expecting proof index of 1")
	}
	if numLeaves != 2 {
		t.Errorf("computing row proof for (1, 1) in 2x2 square failed; expecting number of leaves to be 2")
	}

	result, err = newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}})
	if err != nil {
		panic(err)
	}
	_, proof, proofIndex, numLeaves = result.computeColumnProof(1, 1)
	if len(proof) != 2 {
		t.Errorf("computing column proof for (1, 1) in 2x2 square failed; expecting proof set of length 2")
	}
	if proofIndex != 1 {
		t.Errorf("computing column proof for (1, 1) in 2x2 square failed; expecting proof index of 1")
	}
	if numLeaves != 2 {
		t.Errorf("computing column proof for (1, 1) in 2x2 square failed; expecting number of leaves to be 2")
	}
}
