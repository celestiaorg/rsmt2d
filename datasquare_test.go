package rsmt2d

import (
	"fmt"
	"reflect"
	"testing"
)

func TestNewDataSquare(t *testing.T) {
	result, err := newDataSquare([][]byte{{1, 2}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(result.squareRow, [][][]byte{{{1, 2}}}) {
		t.Errorf("newDataSquare failed for 1x1 square")
	}

	result, err = newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(result.squareRow, [][][]byte{{{1, 2}, {3, 4}}, {{5, 6}, {7, 8}}}) {
		t.Errorf("newDataSquare failed for 2x2 square")
	}

	_, err = newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}}, NewDefaultTree)
	if err == nil {
		t.Errorf("newDataSquare failed; inconsistent number of chunks accepted")
	}

	_, err = newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7}}, NewDefaultTree)
	if err == nil {
		t.Errorf("newDataSquare failed; chunks of unequal size accepted")
	}
}

func TestExtendSquare(t *testing.T) {
	ds, err := newDataSquare([][]byte{{1, 2}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	err = ds.extendSquare(1, []byte{0})
	if err == nil {
		t.Errorf("extendSquare failed; error not returned when filler chunk size does not match data square chunk size")
	}

	ds, err = newDataSquare([][]byte{{1, 2}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	err = ds.extendSquare(1, []byte{0, 0})
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(ds.squareRow, [][][]byte{{{1, 2}, {0, 0}}, {{0, 0}, {0, 0}}}) {
		t.Errorf("extendSquare failed; unexpected result when extending 1x1 square to 2x2 square")
	}
}

func TestRoots(t *testing.T) {
	result, err := newDataSquare([][]byte{{1, 2}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(result.RowRoots(), result.ColumnRoots()) {
		t.Errorf("computing roots failed; expecting row and column roots for 1x1 square to be equal")
	}
}

func TestLazyRootGeneration(t *testing.T) {
	square, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	var rowRoots [][]byte
	var colRoots [][]byte

	for i := uint(0); i < square.width; i++ {
		rowRoots = append(rowRoots, square.RowRoot(i))
		colRoots = append(rowRoots, square.ColRoot(i))
	}

	square.computeRoots()

	if !reflect.DeepEqual(square.rowRoots, rowRoots) && !reflect.DeepEqual(square.columnRoots, colRoots) {
		t.Error("RowRoot or ColumnRoot did not produce identical roots to computeRoots")
	}
}

func TestRootAPI(t *testing.T) {
	square, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	for i := uint(0); i < square.width; i++ {
		if !reflect.DeepEqual(square.RowRoots()[i], square.RowRoot(i)) {
			t.Errorf(
				"Row root API results in different roots, expected %v go %v",
				square.RowRoots()[i],
				square.RowRoot(i),
			)
		}
		if !reflect.DeepEqual(square.ColumnRoots()[i], square.ColRoot(i)) {
			t.Errorf(
				"Column root API results in different roots, expected %v go %v",
				square.ColumnRoots()[i],
				square.ColRoot(i),
			)
		}
	}
}

func TestProofs(t *testing.T) {
	result, err := newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	_, proof, proofIndex, numLeaves, err := result.computeRowProof(1, 1)
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	if len(proof) != 2 {
		t.Errorf("computing row proof for (1, 1) in 2x2 square failed; expecting proof set of length 2")
	}
	if proofIndex != 1 {
		t.Errorf("computing row proof for (1, 1) in 2x2 square failed; expecting proof index of 1")
	}
	if numLeaves != 2 {
		t.Errorf("computing row proof for (1, 1) in 2x2 square failed; expecting number of leaves to be 2")
	}

	result, err = newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	_, proof, proofIndex, numLeaves, err = result.computeColumnProof(1, 1)
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
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

func BenchmarkRoots(b *testing.B) {
	for i := 32; i < 257; i *= 2 {
		square, err := newDataSquare(genRandDS(i), NewDefaultTree)
		if err != nil {
			b.Errorf("Failure to create square of size %d: %s", i, err)
		}
		b.Run(
			fmt.Sprintf("Square Size %dx%d", i, i),
			func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					square.computeRoots()
				}
			},
		)
	}
}
