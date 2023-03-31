package rsmt2d

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDataSquare(t *testing.T) {
	tests := []struct {
		name     string
		cells    [][]byte
		expected [][][]byte
	}{
		{"1x1", [][]byte{{1, 2}}, [][][]byte{{{1, 2}}}},
		{"2x2", [][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}}, [][][]byte{{{1, 2}, {3, 4}}, {{5, 6}, {7, 8}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := newDataSquare(test.cells, NewDefaultTree)
			if err != nil {
				panic(err)
			}
			if !reflect.DeepEqual(result.squareRow, test.expected) {
				t.Errorf("newDataSquare failed for %v square", test.name)
			}
		})
	}
}

func TestInvalidDataSquareCreation(t *testing.T) {
	tests := []struct {
		name  string
		cells [][]byte
	}{
		{"InconsistentChunkNumber", [][]byte{{1, 2}, {3, 4}, {5, 6}}},
		{"UnequalChunkSize", [][]byte{{1, 2}, {3, 4}, {5, 6}, {7}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := newDataSquare(test.cells, NewDefaultTree)
			if err == nil {
				t.Errorf("newDataSquare failed; chunks accepted with %v", test.name)
			}
		})
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
	err = ds.extendSquare(1, []byte{0, 0})
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(ds.squareRow, [][][]byte{{{1, 2}, {0, 0}}, {{0, 0}, {0, 0}}}) {
		t.Errorf("extendSquare failed; unexpected result when extending 1x1 square to 2x2 square")
	}
}

func TestInvalidSquareExtension(t *testing.T) {
	ds, err := newDataSquare([][]byte{{1, 2}}, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	err = ds.extendSquare(1, []byte{0})
	if err == nil {
		t.Errorf("extendSquare failed; error not returned when filler chunk size does not match data square chunk size")
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
		rowRoot, err := square.getRowRoot(i)
		assert.NoError(t, err)
		colRoot, err := square.getColRoot(i)
		assert.NoError(t, err)
		rowRoots = append(rowRoots, rowRoot)
		colRoots = append(colRoots, colRoot)
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
		rowRoot, err := square.getRowRoot(i)
		assert.NoError(t, err)
		if !reflect.DeepEqual(square.getRowRoots()[i], rowRoot) {
			t.Errorf(
				"Row root API results in different roots, expected %v got %v",
				square.getRowRoots()[i],
				rowRoot,
			)
		}
		colRoot, err := square.getColRoot(i)
		assert.NoError(t, err)
		if !reflect.DeepEqual(square.getColRoots()[i], colRoot) {
			t.Errorf(
				"Column root API results in different roots, expected %v got %v",
				square.getColRoots()[i],
				colRoot,
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
}

func BenchmarkEDSRoots(b *testing.B) {
	for i := 32; i < 513; i *= 2 {
		square, err := newDataSquare(genRandDS(i*2), NewDefaultTree)
		if err != nil {
			b.Errorf("Failure to create square of size %d: %s", i, err)
		}
		b.Run(
			fmt.Sprintf("%dx%dx%d ODS", i, i, int(square.chunkSize)),
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
	tree := ds.createTreeFn(Row, x)
	data := ds.row(x)

	for i := uint(0); i < ds.width; i++ {
		tree.Push(data[i])
	}

	merkleRoot, proof, proofIndex, numLeaves := treeProve(tree.(*DefaultTree), int(y))
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
