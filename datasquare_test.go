package rsmt2d

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"testing"

	"github.com/celestiaorg/merkletree"
	"github.com/stretchr/testify/assert"
)

func TestNewDataSquare(t *testing.T) {
	tests := []struct {
		name      string
		cells     [][]byte
		expected  [][][]byte
		chunkSize uint
	}{
		{"1x1", [][]byte{{1, 2}}, [][][]byte{{{1, 2}}}, 2},
		{"2x2", [][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}}, [][][]byte{{{1, 2}, {3, 4}}, {{5, 6}, {7, 8}}}, 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := newDataSquare(test.cells, NewDefaultTree, test.chunkSize)
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
		name      string
		cells     [][]byte
		chunkSize uint
	}{
		{"InconsistentChunkNumber", [][]byte{{1, 2}, {3, 4}, {5, 6}}, 2},
		{"UnequalChunkSize", [][]byte{{1, 2}, {3, 4}, {5, 6}, {7}}, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := newDataSquare(test.cells, NewDefaultTree, test.chunkSize)
			if err == nil {
				t.Errorf("newDataSquare failed; chunks accepted with %v", test.name)
			}
		})
	}
}

func TestSetCell(t *testing.T) {
	type testCase struct {
		name         string
		originalCell []byte
		newCell      []byte
		wantErr      bool
	}

	testCases := []testCase{
		{
			name:         "can set cell if originally nil",
			originalCell: nil,
			newCell:      []byte{42},
			wantErr:      false,
		},
		{
			name:         "expect error if cell is not originally nil",
			originalCell: []byte{1},
			wantErr:      true,
		},
		{
			name:         "expect error if new cell is not the correct chunk size",
			originalCell: nil,
			newCell:      []byte{1, 2}, // incorrect chunk size
			wantErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ds, err := newDataSquare([][]byte{tc.originalCell, {2}, {3}, {4}}, NewDefaultTree, 1)
			assert.NoError(t, err)

			err = ds.SetCell(0, 0, tc.newCell)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.newCell, ds.GetCell(0, 0))
			}
		})
	}
}

// Test_setCell verifies that setCell can overwrite cells without performing any
// input validation.
func Test_setCell(t *testing.T) {
	type testCase struct {
		name     string
		original []byte
		new      []byte
	}

	testCases := []testCase{
		{
			name:     "can set cell if originally nil",
			original: nil,
			new:      []byte{42},
		},
		{
			name:     "can set cell if originally some value",
			original: []byte{1},
			new:      []byte{42},
		},
		{
			name:     "can set cell to nil",
			original: []byte{1},
			new:      nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ds, err := newDataSquare([][]byte{tc.original, {2}, {3}, {4}}, NewDefaultTree, 1)
			assert.NoError(t, err)

			ds.setCell(0, 0, tc.new)
			assert.Equal(t, tc.new, ds.GetCell(0, 0))
		})
	}
}

func TestGetCell(t *testing.T) {
	ds, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree, 1)
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
	ds, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree, 1)
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
	ds, err := newDataSquare([][]byte{{1, 2}}, NewDefaultTree, 2)
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
	ds, err := newDataSquare([][]byte{{1, 2}}, NewDefaultTree, 2)
	if err != nil {
		panic(err)
	}
	err = ds.extendSquare(1, []byte{0})
	if err == nil {
		t.Errorf("extendSquare failed; error not returned when filler chunk size does not match data square chunk size")
	}
}

// TestRoots verifies that the row roots and column roots are equal for a 1x1
// square.
func TestRoots(t *testing.T) {
	result, err := newDataSquare([][]byte{{1, 2}}, NewDefaultTree, 2)
	assert.NoError(t, err)

	rowRoots, err := result.getRowRoots()
	assert.NoError(t, err)

	colRoots, err := result.getColRoots()
	assert.NoError(t, err)

	assert.Equal(t, rowRoots, colRoots)
}

func TestLazyRootGeneration(t *testing.T) {
	square, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree, 1)
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

	err = square.computeRoots()
	assert.NoError(t, err)

	assert.Equal(t, square.rowRoots, rowRoots)
	assert.Equal(t, square.colRoots, colRoots)
}

func TestComputeRoots(t *testing.T) {
	t.Run("default tree computeRoots() returns no error", func(t *testing.T) {
		square, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree, 1)
		assert.NoError(t, err)
		err = square.computeRoots()
		assert.NoError(t, err)
	})
	t.Run("error tree computeRoots() returns an error", func(t *testing.T) {
		square, err := newDataSquare([][]byte{{1}}, newErrorTree, 1)
		assert.NoError(t, err)
		err = square.computeRoots()
		assert.Error(t, err)
	})
}

func TestRootAPI(t *testing.T) {
	square, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree, 1)
	if err != nil {
		panic(err)
	}

	for i := uint(0); i < square.width; i++ {
		rowRoot, err := square.getRowRoot(i)
		assert.NoError(t, err)

		rowRoots, err := square.getRowRoots()
		assert.NoError(t, err)

		assert.Equal(t, rowRoots[i], rowRoot)

		colRoot, err := square.getColRoot(i)
		assert.NoError(t, err)

		colRoots, err := square.getColRoots()
		assert.NoError(t, err)

		assert.Equal(t, colRoots[i], colRoot)
	}
}

func TestDefaultTreeProofs(t *testing.T) {
	result, err := newDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}}, NewDefaultTree, 2)
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

func Test_setRowSlice(t *testing.T) {
	type testCase struct {
		name    string
		newRow  [][]byte
		x       uint
		y       uint
		want    [][]byte
		wantErr bool
	}
	testCases := []testCase{
		{
			name:    "overwrite the first row",
			newRow:  [][]byte{{5}, {6}},
			x:       0,
			y:       0,
			want:    [][]byte{{5}, {6}, {3}, {4}},
			wantErr: false,
		},
		{
			name:    "overwrite the last row",
			newRow:  [][]byte{{5}, {6}},
			x:       1,
			y:       0,
			want:    [][]byte{{1}, {2}, {5}, {6}},
			wantErr: false,
		},
		{
			name:    "returns an error if the new row has an invalid chunk size",
			newRow:  [][]byte{{5, 6}},
			x:       0,
			y:       0,
			wantErr: true,
		},
		{
			name:    "returns an error if the new row would surpass the data square's width",
			newRow:  [][]byte{{5}, {6}},
			x:       0,
			y:       1,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		ds, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree, 1)
		assert.NoError(t, err)
		err = ds.setRowSlice(tc.x, tc.y, tc.newRow)

		if tc.wantErr {
			assert.Error(t, err)
			return
		}
		assert.NoError(t, err)
		assert.Equal(t, tc.want, ds.Flattened())
	}
}

func Test_setColSlice(t *testing.T) {
	type testCase struct {
		name    string
		newCol  [][]byte
		x       uint
		y       uint
		want    [][]byte
		wantErr bool
	}
	testCases := []testCase{
		{
			name:    "overwrite the first col",
			newCol:  [][]byte{{5}, {6}},
			x:       0,
			y:       0,
			want:    [][]byte{{5}, {2}, {6}, {4}},
			wantErr: false,
		},
		{
			name:    "overwrite the last col",
			newCol:  [][]byte{{5}, {6}},
			x:       0,
			y:       1,
			want:    [][]byte{{1}, {5}, {3}, {6}},
			wantErr: false,
		},
		{
			name:    "returns an error if the new col has an invalid chunk size",
			newCol:  [][]byte{{5, 6}},
			x:       0,
			y:       0,
			wantErr: true,
		},
		{
			name:    "returns an error if the new col would surpass the data square's width",
			newCol:  [][]byte{{5}, {6}},
			x:       1,
			y:       0,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		ds, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree, 1)
		assert.NoError(t, err)
		err = ds.setColSlice(tc.x, tc.y, tc.newCol)

		if tc.wantErr {
			assert.Error(t, err)
			return
		}
		assert.NoError(t, err)
		assert.Equal(t, tc.want, ds.Flattened())
	}
}

func BenchmarkEDSRoots(b *testing.B) {
	for i := 32; i < 513; i *= 2 {
		square, err := newDataSquare(genRandDS(i*2, int(shareSize)), NewDefaultTree, shareSize)
		if err != nil {
			b.Errorf("Failure to create square of size %d: %s", i, err)
		}
		b.Run(
			fmt.Sprintf("%dx%dx%d ODS", i, i, int(square.chunkSize)),
			func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					square.resetRoots()
					err := square.computeRoots()
					assert.NoError(b, err)
				}
			},
		)
	}
}

func computeRowProof(ds *dataSquare, x uint, y uint) ([]byte, [][]byte, uint, uint, error) {
	tree := ds.createTreeFn(Row, x)
	data := ds.row(x)

	for i := uint(0); i < ds.width; i++ {
		err := tree.Push(data[i])
		if err != nil {
			return nil, nil, 0, 0, err
		}
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

type errorTree struct {
	*merkletree.Tree
	leaves [][]byte
}

func newErrorTree(_ Axis, _ uint) Tree {
	return &errorTree{
		Tree:   merkletree.New(sha256.New()),
		leaves: make([][]byte, 0, 128),
	}
}

func (d *errorTree) Push(data []byte) error {
	// ignore the idx, as this implementation doesn't need that info
	d.leaves = append(d.leaves, data)
	return nil
}

func (d *errorTree) Root() ([]byte, error) {
	return nil, fmt.Errorf("error")
}

// setCell overwrites the contents of a specific cell. setCell does not perform
// any input validation so most use cases should use `SetCell` instead of
// `setCell`. This method exists strictly for testing.
func (ds *dataSquare) setCell(x uint, y uint, newChunk []byte) {
	ds.squareRow[x][y] = newChunk
	ds.squareCol[y][x] = newChunk
	ds.resetRoots()
}
