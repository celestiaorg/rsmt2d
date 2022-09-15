package rsmt2d

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestSetCell(t *testing.T) {
	ds, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	// SetCell can only write to nil cells
	assert.Panics(t, func() { ds.SetCell(0, 0, []byte{0}) })

	// Set the cell to nil to allow modification
	ds.setCell(0, 0, nil)

	ds.SetCell(0, 0, []byte{42})
	assert.Equal(t, []byte{42}, ds.GetCell(0, 0))
}

func TestGetCell(t *testing.T) {
	ds, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	cell := ds.GetCell(0, 0)
	cell[0] = 42

	if reflect.DeepEqual(ds.GetCell(0, 0), []byte{42}) {
		t.Errorf("GetCell failed to return an immutable copy of the cell")
	}
}

func TestFlattened(t *testing.T) {
	ds, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	flattened := ds.Flattened()
	flattened[0] = []byte{42}

	if reflect.DeepEqual(ds.Flattened(), [][]byte{{42}, {2}, {3}, {4}}) {
		t.Errorf("Flattened failed to return an immutable copy")
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
	if !reflect.DeepEqual(result.getRowRoots(), result.getColRoots()) {
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
		rowRoots = append(rowRoots, square.getRowRoot(i))
		colRoots = append(rowRoots, square.getColRoot(i))
	}

	square.computeRoots()

	if !reflect.DeepEqual(square.rowRoots, rowRoots) && !reflect.DeepEqual(square.colRoots, colRoots) {
		t.Error("getRowRoot or getColRoot did not produce identical roots to computeRoots")
	}
}

func TestRootAPI(t *testing.T) {
	square, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	for i := uint(0); i < square.width; i++ {
		if !reflect.DeepEqual(square.getRowRoots()[i], square.getRowRoot(i)) {
			t.Errorf(
				"Row root API results in different roots, expected %v go %v",
				square.getRowRoots()[i],
				square.getRowRoot(i),
			)
		}
		if !reflect.DeepEqual(square.getColRoots()[i], square.getColRoot(i)) {
			t.Errorf(
				"Column root API results in different roots, expected %v go %v",
				square.getColRoots()[i],
				square.getColRoot(i),
			)
		}
	}
}

func TestDefaultTreeProofs(t *testing.T) {
	result, err := newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	_, proof, proofIndex, numLeaves, err := computeRowProof(result, 1, 1)
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
	_, proof, proofIndex, numLeaves, err = computeColProof(result, 1, 1)
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

func BenchmarkEDSRoots(b *testing.B) {
	for i := 32; i < 513; i *= 2 {
		square, err := newDataSquare(genRandDS(i*2), NewDefaultTree)
		if err != nil {
			b.Errorf("Failure to create square of size %d: %s", i, err)
		}
		b.Run(
			fmt.Sprintf("%dx%dx256 ODS", i, i),
			func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					square.resetRoots()
					square.computeRoots()
				}
			},
		)
	}
}

func computeRowProof(ds *dataSquare, x uint, y uint) ([]byte, [][]byte, uint, uint, error) {
	tree := ds.createTreeFn()
	data := ds.row(x)

	for i := uint(0); i < ds.width; i++ {
		tree.Push(data[i], SquareIndex{Axis: y, Cell: uint(i)})
	}

	merkleRoot, proof, proofIndex, numLeaves := treeProve(tree.(*DefaultTree), int(y))
	return merkleRoot, proof, uint(proofIndex), uint(numLeaves), nil
}

func computeColProof(ds *dataSquare, x uint, y uint) ([]byte, [][]byte, uint, uint, error) {
	tree := ds.createTreeFn()
	data := ds.col(y)

	for i := uint(0); i < ds.width; i++ {
		tree.Push(data[i], SquareIndex{Axis: y, Cell: uint(i)})
	}
	// TODO(ismail): check for overflow when casting from uint -> int
	merkleRoot, proof, proofIndex, numLeaves := treeProve(tree.(*DefaultTree), int(x))
	return merkleRoot, proof, uint(proofIndex), uint(numLeaves), nil
}

func treeProve(d *DefaultTree, idx int) (merkleRoot []byte, proofSet [][]byte, proofIndex uint64, numLeaves uint64) {
	if err := d.Tree.SetIndex(uint64(idx)); err != nil {
		panic(fmt.Sprintf("don't call prove on a already used tree: %v", err))
	}
	for _, l := range d.leaves {
		d.Tree.Push(l)
	}
	return d.Tree.Prove()
}
