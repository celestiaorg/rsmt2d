package rsmt2d

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubCodec wraps the Leopard codec so tests can inject failures into
// individual methods while every other method behaves like the real codec.
type stubCodec struct {
	inner       Codec
	encodeErr   error
	decodeErr   error
	validateErr error
	maxChunks   int // 0 means defer to inner
}

func newStubCodec() *stubCodec {
	return &stubCodec{inner: NewLeoRSCodec()}
}

func (c *stubCodec) Encode(data [][]byte) ([][]byte, error) {
	if c.encodeErr != nil {
		return nil, c.encodeErr
	}
	return c.inner.Encode(data)
}

func (c *stubCodec) Decode(data [][]byte) ([][]byte, error) {
	if c.decodeErr != nil {
		return nil, c.decodeErr
	}
	return c.inner.Decode(data)
}

func (c *stubCodec) MaxChunks() int {
	if c.maxChunks > 0 {
		return c.maxChunks
	}
	return c.inner.MaxChunks()
}

func (c *stubCodec) Name() string { return "stubCodec" }

func (c *stubCodec) ValidateChunkSize(chunkSize int) error {
	if c.validateErr != nil {
		return c.validateErr
	}
	return c.inner.ValidateChunkSize(chunkSize)
}

// failingTree wraps DefaultTree and fails Push on the failPushAt-th call
// (0-indexed; -1 never fails) and/or fails Root.
type failingTree struct {
	inner      Tree
	failPushAt int
	pushes     int
	rootErr    error
}

var errTreeFailure = errors.New("tree failure")

func (ft *failingTree) Push(data []byte) error {
	defer func() { ft.pushes++ }()
	if ft.failPushAt >= 0 && ft.pushes == ft.failPushAt {
		return errTreeFailure
	}
	return ft.inner.Push(data)
}

func (ft *failingTree) Root() ([]byte, error) {
	if ft.rootErr != nil {
		return nil, ft.rootErr
	}
	return ft.inner.Root()
}

// failingTreeFor returns a TreeConstructorFn that builds a DefaultTree for
// every axis and index except the one given, which gets a failingTree.
func failingTreeFor(axis Axis, index uint, failPushAt int, rootErr error) TreeConstructorFn {
	return func(a Axis, i uint) Tree {
		if a == axis && i == index {
			return &failingTree{inner: NewDefaultTree(a, i), failPushAt: failPushAt, rootErr: rootErr}
		}
		return NewDefaultTree(a, i)
	}
}

func TestRegisterCodecPanicsOnDuplicate(t *testing.T) {
	const name = "registerCodecTest"
	t.Cleanup(func() { delete(codecs, name) })

	registerCodec(name, newStubCodec())
	require.NotNil(t, codecs[name])
	assert.Panics(t, func() { registerCodec(name, newStubCodec()) })
}

func TestSetSliceRejectsOverflow(t *testing.T) {
	ds, err := newDataSquare([][]byte{{1}, {2}, {3}, {4}}, NewDefaultTree, 1)
	require.NoError(t, err)

	err = ds.setRowSlice(0, 1, [][]byte{{5}, {6}})
	require.ErrorContains(t, err, "exceed the data square width")

	err = ds.setColSlice(0, 1, [][]byte{{5}, {6}})
	require.ErrorContains(t, err, "exceed the data square width")

	// The square must be untouched after the rejected writes.
	assert.Equal(t, [][]byte{{1}, {2}, {3}, {4}}, ds.Flattened())
}

func TestGetShareSizeReturnsZeroWithoutShares(t *testing.T) {
	assert.Equal(t, 0, getShareSize(nil))
	assert.Equal(t, 0, getShareSize([][]byte{nil, nil}))
	assert.Equal(t, 2, getShareSize([][]byte{nil, {1, 2}}))
}

func TestEqualsReturnsFalseForUnequalWidth(t *testing.T) {
	codec := NewLeoRSCodec()
	a, err := NewExtendedDataSquare(codec, NewDefaultTree, 4, shareSize)
	require.NoError(t, err)
	b, err := NewExtendedDataSquare(codec, NewDefaultTree, 8, shareSize)
	require.NoError(t, err)
	// Match every field Equals checks before width so the width comparison is
	// the one that decides.
	b.originalDataWidth = a.originalDataWidth

	assert.False(t, a.Equals(b))
	assert.False(t, b.Equals(a))
}

func TestComputeExtendedDataSquareErrors(t *testing.T) {
	data := [][]byte{ones, twos, threes, fours}

	t.Run("exceeds max chunks", func(t *testing.T) {
		codec := newStubCodec()
		codec.maxChunks = 1
		_, err := ComputeExtendedDataSquare(data, codec, NewDefaultTree)
		require.ErrorContains(t, err, "exceeds the maximum")
	})
	t.Run("invalid chunk size", func(t *testing.T) {
		codec := newStubCodec()
		codec.validateErr = errors.New("bad chunk size")
		_, err := ComputeExtendedDataSquare(data, codec, NewDefaultTree)
		require.ErrorIs(t, err, codec.validateErr)
	})
	t.Run("encode failure propagates", func(t *testing.T) {
		codec := newStubCodec()
		codec.encodeErr = errors.New("encode failure")
		_, err := ComputeExtendedDataSquare(data, codec, NewDefaultTree)
		require.ErrorIs(t, err, codec.encodeErr)
	})
}

func TestImportExtendedDataSquareErrors(t *testing.T) {
	t.Run("exceeds max chunks", func(t *testing.T) {
		codec := newStubCodec()
		codec.maxChunks = 1
		_, err := ImportExtendedDataSquare(make([][]byte, 16), codec, NewDefaultTree)
		require.ErrorContains(t, err, "exceeds the maximum")
	})
	t.Run("invalid chunk size", func(t *testing.T) {
		codec := newStubCodec()
		codec.validateErr = errors.New("bad chunk size")
		_, err := ImportExtendedDataSquare([][]byte{ones, twos, threes, fours}, codec, NewDefaultTree)
		require.ErrorIs(t, err, codec.validateErr)
	})
	t.Run("not a square", func(t *testing.T) {
		_, err := ImportExtendedDataSquare([][]byte{ones, twos, threes}, NewLeoRSCodec(), NewDefaultTree)
		require.ErrorContains(t, err, "square number")
	})
	t.Run("odd width", func(t *testing.T) {
		_, err := ImportExtendedDataSquare([][]byte{ones, twos, threes, fours, ones, twos, threes, fours, ones}, NewLeoRSCodec(), NewDefaultTree)
		require.ErrorContains(t, err, "must be even")
	})
	t.Run("uneven shares that pass chunk size validation", func(t *testing.T) {
		// Only the first non-nil share is validated against the codec, so a
		// later share of a different (but codec-valid) size must be caught by
		// the data square constructor.
		doubleSize := bytes.Repeat([]byte{4}, 2*shareSize)
		_, err := ImportExtendedDataSquare([][]byte{ones, twos, threes, doubleSize}, NewLeoRSCodec(), NewDefaultTree)
		require.ErrorIs(t, err, ErrUnevenChunks)
	})
}

func TestNewExtendedDataSquareRejectsInvalidChunkSize(t *testing.T) {
	codec := newStubCodec()
	codec.validateErr = errors.New("bad chunk size")
	_, err := NewExtendedDataSquare(codec, NewDefaultTree, 4, shareSize)
	require.ErrorIs(t, err, codec.validateErr)
}

func TestRootsPropagatesTreeErrors(t *testing.T) {
	data := [][]byte{ones, twos, threes, fours}

	// computeRoots builds row and column roots together, so a failure on
	// either axis surfaces from the first RowRoots call inside Roots.
	for _, axis := range []Axis{Row, Col} {
		t.Run(axis.String()+" tree failure", func(t *testing.T) {
			eds, err := ComputeExtendedDataSquare(data, NewLeoRSCodec(), failingTreeFor(axis, 1, -1, errTreeFailure))
			require.NoError(t, err)
			_, err = eds.Roots()
			require.ErrorIs(t, err, errTreeFailure)
		})
	}
}

// repairWithTree takes the honest roots of a fresh EDS, blanks the given
// cells, re-imports the square with treeFn, and runs Repair.
func repairWithTree(t *testing.T, treeFn TreeConstructorFn, missing [][2]uint) error {
	t.Helper()
	codec := NewLeoRSCodec()
	original := createTestEds(codec, shareSize)
	rowRoots, err := original.getRowRoots()
	require.NoError(t, err)
	colRoots, err := original.getColRoots()
	require.NoError(t, err)

	eds, err := ImportExtendedDataSquare(original.Flattened(), codec, treeFn)
	require.NoError(t, err)
	for _, cell := range missing {
		eds.setCell(cell[0], cell[1], nil)
	}
	return eds.Repair(rowRoots, colRoots)
}

func TestRepairTreatsRootComputationFailureAsByzantine(t *testing.T) {
	t.Run("row solve, Push fails", func(t *testing.T) {
		// Row 0 is missing two shares and is rebuilt first; its root is
		// computed with a tree whose first Push fails.
		err := repairWithTree(t, failingTreeFor(Row, 0, 0, nil), [][2]uint{{0, 2}, {0, 3}})
		var byzData *ErrByzantineData
		require.ErrorAs(t, err, &byzData)
		assert.Equal(t, Row, byzData.Axis)
		assert.Equal(t, uint(0), byzData.Index)
	})
	t.Run("col solve, Root fails", func(t *testing.T) {
		// Row 0 keeps a single share so it cannot be decoded; column 0 is then
		// rebuilt and its root computation fails.
		err := repairWithTree(t, failingTreeFor(Col, 0, -1, errTreeFailure), [][2]uint{{0, 1}, {0, 2}, {0, 3}, {2, 0}})
		var byzData *ErrByzantineData
		require.ErrorAs(t, err, &byzData)
		assert.Equal(t, Col, byzData.Axis)
		assert.Equal(t, uint(0), byzData.Index)
	})
}

func TestRepairOrthogonalRootComputationFailures(t *testing.T) {
	// Removing (1, 2) and (2, 1) leaves row 0 and column 0 complete, so the
	// first rebuild is row 1, whose rebuilt share (1, 2) completes column 2.
	// Column 2's root is then computed with the rebuilt share spliced in at
	// index 1, so a tree for (Col, 2) can be made to fail before, at, or after
	// the rebuilt index.
	missing := [][2]uint{{1, 2}, {2, 1}}
	for _, tc := range []struct {
		name       string
		failPushAt int
	}{
		{"push fails before rebuilt index", 0},
		{"push fails at rebuilt index", 1},
		{"push fails after rebuilt index", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := repairWithTree(t, failingTreeFor(Col, 2, tc.failPushAt, nil), missing)
			var byzData *ErrByzantineData
			require.ErrorAs(t, err, &byzData)
			assert.Equal(t, Col, byzData.Axis)
			assert.Equal(t, uint(2), byzData.Index)
		})
	}
}

func TestErrByzantineDataOnOrthogonalRowDuringColumnSolve(t *testing.T) {
	corruptShare := bytes.Repeat([]byte{66}, shareSize)
	codec := NewLeoRSCodec()

	eds := createTestEds(codec, shareSize)
	rowRoots, err := eds.getRowRoots()
	require.NoError(t, err)
	colRoots, err := eds.getColRoots()
	require.NoError(t, err)

	// Transpose of TestErrByzantineDataSharesMatchOrthogonalAxis. Row 0 keeps a
	// single share so it cannot be decoded, which forces column 0 to be the
	// first vector rebuilt. Rebuilding (2, 0) completes row 2, whose corrupt
	// (2, 2) makes it fail its row root, so the error names row 2 (orthogonal
	// to the column solve in progress) and carries row 2's shares.
	//
	// O _ _ _      _ = nil share
	// O O O O      C = corrupted share
	// _ O C O      O = original/parity share
	// O O O O
	eds.setCell(0, 1, nil)
	eds.setCell(0, 2, nil)
	eds.setCell(0, 3, nil)
	eds.setCell(2, 0, nil)
	eds.setCell(2, 2, corruptShare)

	err = eds.Repair(rowRoots, colRoots)
	var byzData *ErrByzantineData
	require.ErrorAs(t, err, &byzData)
	require.Equal(t, Row, byzData.Axis, "Byzantine error must be on the row axis")
	require.Equal(t, uint(2), byzData.Index, "Byzantine error must be on row index 2")

	require.Equal(t, int(eds.Width()), len(byzData.Shares))
	assert.Contains(t, byzData.Shares, corruptShare,
		"Shares must contain the corrupt cell that lives on the byzantine (row) axis")
	assert.Nil(t, byzData.Shares[0], "missing share at the rebuilt index must remain nil")
}

func TestRepairSurfacesEncodeFailureDuringVerification(t *testing.T) {
	codec := newStubCodec()
	original := createTestEds(codec, shareSize)
	rowRoots, err := original.getRowRoots()
	require.NoError(t, err)
	colRoots, err := original.getColRoots()
	require.NoError(t, err)

	eds, err := ImportExtendedDataSquare(original.Flattened(), codec, NewDefaultTree)
	require.NoError(t, err)
	eds.setCell(0, 0, nil)

	// Every re-encoding check now fails, so Repair cannot vouch for any
	// complete vector and reports the failure as byzantine data.
	codec.encodeErr = errors.New("encode failure")
	err = eds.Repair(rowRoots, colRoots)
	var byzData *ErrByzantineData
	require.ErrorAs(t, err, &byzData)
}

// badlyEncodedColumnSquare is the transpose of the square crafted in
// TestRepairRejectsBadlyEncodedSelfDecode: every row is a valid codeword but
// column 0 is not. It returns the crafted square together with the roots a
// malicious proposer would commit to.
func badlyEncodedColumnSquare(t *testing.T, codec Codec) (bad *ExtendedDataSquare, rowRoots, colRoots [][]byte) {
	t.Helper()
	const width = 4

	// column 0 self-decodes from its first three shares but is not a codeword.
	c0, c1, c2 := randShare(t), randShare(t), randShare(t)
	dec, err := codec.Decode([][]byte{c0, c1, c2, nil})
	require.NoError(t, err)
	col0 := [][]byte{c0, c1, c2, dec[3]}

	// column 1 is a genuine codeword.
	r0, r1 := randShare(t), randShare(t)
	p1 := parityShares(t, codec, [][]byte{r0, r1})
	col1 := [][]byte{r0, r1, p1[0], p1[1]}

	// columns 2 and 3 are the row-extension of columns 0 and 1, so every row
	// is a codeword.
	cells := make([][]byte, width*width)
	for r := range width {
		cells[r*width+0] = col0[r]
		cells[r*width+1] = col1[r]
		rowPar := parityShares(t, codec, [][]byte{col0[r], col1[r]})
		cells[r*width+2] = rowPar[0]
		cells[r*width+3] = rowPar[1]
	}

	bad, err = ImportExtendedDataSquare(cells, codec, NewDefaultTree)
	require.NoError(t, err)
	rowRoots, err = bad.RowRoots()
	require.NoError(t, err)
	colRoots, err = bad.ColRoots()
	require.NoError(t, err)

	require.False(t, isCodeword(t, codec, bad.Col(0)), "column 0 must not be a codeword")
	for r := range uint(width) {
		require.True(t, isCodeword(t, codec, bad.Row(r)), "row %d must be a codeword", r)
	}
	return bad, rowRoots, colRoots
}

// repairFromCells rebuilds a fresh square from only the listed cells of src
// and runs Repair against the given roots.
func repairFromCells(t *testing.T, codec Codec, src *ExtendedDataSquare, rowRoots, colRoots [][]byte, cells [][2]uint) error {
	t.Helper()
	eds, err := NewExtendedDataSquare(codec, NewDefaultTree, src.Width(), shareSize)
	require.NoError(t, err)
	for _, c := range cells {
		require.NoError(t, eds.SetCell(c[0], c[1], src.GetCell(c[0], c[1])))
	}
	return eds.Repair(rowRoots, colRoots)
}

func TestRepairRejectsBadlyEncodedColumn(t *testing.T) {
	codec := NewLeoRSCodec()
	bad, rowRoots, colRoots := badlyEncodedColumnSquare(t, codec)

	t.Run("column self-decodes", func(t *testing.T) {
		// Row 0 is rebuilt first and passes (rows are codewords) without
		// completing any column. Column 0 then self-decodes from
		// (0,0), (1,0), (2,0); it matches its committed root but fails the
		// re-encoding check, so the column solve must report it as byzantine.
		err := repairFromCells(t, codec, bad, rowRoots, colRoots,
			[][2]uint{{0, 0}, {0, 1}, {1, 0}, {1, 1}, {2, 0}, {2, 2}})
		var byzData *ErrByzantineData
		require.ErrorAs(t, err, &byzData)
		assert.Equal(t, Col, byzData.Axis)
		assert.Equal(t, uint(0), byzData.Index)
	})
	t.Run("row solve completes the column", func(t *testing.T) {
		// Column 0 is missing only (0,0). Rebuilding row 0 fills it, so the
		// row solve completes column 0, which matches its committed root but
		// fails the orthogonal re-encoding check.
		err := repairFromCells(t, codec, bad, rowRoots, colRoots,
			[][2]uint{{0, 1}, {0, 2}, {1, 0}, {2, 0}, {3, 0}})
		var byzData *ErrByzantineData
		require.ErrorAs(t, err, &byzData)
		assert.Equal(t, Col, byzData.Axis)
		assert.Equal(t, uint(0), byzData.Index)
	})
}
