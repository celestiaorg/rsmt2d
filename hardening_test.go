package rsmt2d

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrByzantineDataSharesCommittedByRowOrColRoot covers a square whose row
// and column roots disagree at a single coordinate. Repair names the column as
// byzantine, and a share in the error may come from the row root rather than
// the column root. Each non-nil share must still be committed to by either its
// row root or its column root so that a consumer can prove it.
func TestErrByzantineDataSharesCommittedByRowOrColRoot(t *testing.T) {
	codec := NewLeoRSCodec()
	sA := bytes.Repeat([]byte{1}, 64)
	sB := bytes.Repeat([]byte{2}, 64)
	s1 := bytes.Repeat([]byte{3}, 64)
	s2 := bytes.Repeat([]byte{4}, 64)
	s3 := bytes.Repeat([]byte{5}, 64)

	// edsRow and edsCol differ only at (0, 0).
	edsRow, err := ComputeExtendedDataSquare([][]byte{sA, s1, s2, s3}, codec, NewDefaultTree)
	require.NoError(t, err)
	edsCol, err := ComputeExtendedDataSquare([][]byte{sB, s1, s2, s3}, codec, NewDefaultTree)
	require.NoError(t, err)
	rowRoots, err := edsRow.RowRoots()
	require.NoError(t, err)
	colRoots, err := edsCol.ColRoots()
	require.NoError(t, err)

	partial, err := NewExtendedDataSquare(codec, NewDefaultTree, 4, 64)
	require.NoError(t, err)
	require.NoError(t, partial.SetCell(0, 1, edsRow.GetCell(0, 1)))
	require.NoError(t, partial.SetCell(0, 2, edsRow.GetCell(0, 2)))
	require.NoError(t, partial.SetCell(1, 0, edsCol.GetCell(1, 0)))

	err = partial.Repair(rowRoots, colRoots)
	var byzErr *ErrByzantineData
	require.ErrorAs(t, err, &byzErr)
	require.Equal(t, Col, byzErr.Axis)
	require.Equal(t, uint(0), byzErr.Index)
	require.Len(t, byzErr.Shares, 4)

	for rowIdx, share := range byzErr.Shares {
		if share == nil {
			continue
		}
		inRowRoot := bytes.Equal(share, edsRow.GetCell(uint(rowIdx), 0))
		inColRoot := bytes.Equal(share, edsCol.GetCell(uint(rowIdx), 0))
		assert.True(t, inRowRoot || inColRoot,
			"share %d of column 0 must be committed to by its row root or its column root", rowIdx)
	}
}

// TestPreRepairSanityCheckDoesNotAliasEDS ensures that the shares attached to
// an ErrByzantineData returned by preRepairSanityCheck are copies, so that a
// caller mutating them cannot corrupt the EDS.
func TestPreRepairSanityCheckDoesNotAliasEDS(t *testing.T) {
	codec := NewLeoRSCodec()
	eds := createTestEds(codec, shareSize)
	rowRoots, err := eds.getRowRoots()
	require.NoError(t, err)
	colRoots, err := eds.getColRoots()
	require.NoError(t, err)

	// Corrupt every root so that whichever check fails first returns a
	// byzantine error carrying a complete row or column.
	badRowRoots := deepCopy(rowRoots)
	badColRoots := deepCopy(colRoots)
	for i := range badRowRoots {
		badRowRoots[i][len(badRowRoots[i])-1] ^= 0xff
		badColRoots[i][len(badColRoots[i])-1] ^= 0xff
	}

	before := deepCopy(eds.Flattened())
	err = eds.preRepairSanityCheck(badRowRoots, badColRoots)
	var byzErr *ErrByzantineData
	require.ErrorAs(t, err, &byzErr)

	for _, share := range byzErr.Shares {
		require.NotNil(t, share)
		for i := range share {
			share[i] ^= 0xff
		}
	}
	assert.Equal(t, before, eds.Flattened(), "mutating ErrByzantineData.Shares must not modify the EDS")
}

// concurrencyTrackingConstructor is a BufferedTreeConstructor that records the
// maximum number of trees in use at once.
type concurrencyTrackingConstructor struct {
	treeCount int

	mu       sync.Mutex
	inUse    int
	maxInUse int
}

func (c *concurrencyTrackingConstructor) TreeCount() int { return c.treeCount }

func (c *concurrencyTrackingConstructor) NewConstructor(_ uint) TreeConstructorFn {
	return func(axis Axis, index uint) Tree {
		c.mu.Lock()
		c.inUse++
		c.maxInUse = max(c.maxInUse, c.inUse)
		c.mu.Unlock()
		return &trackedTree{Tree: NewDefaultTree(axis, index), done: func() {
			c.mu.Lock()
			c.inUse--
			c.mu.Unlock()
		}}
	}
}

type trackedTree struct {
	Tree
	done func()
}

func (t *trackedTree) Root() ([]byte, error) {
	defer t.done()
	return t.Tree.Root()
}

// TestPreRepairSanityCheckRespectsParallelOps ensures that preRepairSanityCheck
// does not use more trees concurrently than a BufferedTreeConstructor allows.
func TestPreRepairSanityCheckRespectsParallelOps(t *testing.T) {
	codec := NewLeoRSCodec()
	const odsWidth = 16
	data := make([][]byte, odsWidth*odsWidth)
	for i := range data {
		data[i] = bytes.Repeat([]byte{byte(i)}, shareSize)
	}

	reference, err := ComputeExtendedDataSquare(data, codec, NewDefaultTree)
	require.NoError(t, err)
	rowRoots, err := reference.RowRoots()
	require.NoError(t, err)
	colRoots, err := reference.ColRoots()
	require.NoError(t, err)

	constructor := &concurrencyTrackingConstructor{treeCount: 1}
	eds, err := ComputeExtendedDataSquareWithBuffer(data, codec, constructor)
	require.NoError(t, err)

	for i := 0; i < 20; i++ {
		require.NoError(t, eds.preRepairSanityCheck(rowRoots, colRoots))
	}
	assert.LessOrEqual(t, constructor.maxInUse, constructor.treeCount)
}

func TestLeoRSCodecValidateChunkSizeRejectsNonPositive(t *testing.T) {
	codec := NewLeoRSCodec()
	for _, size := range []int{0, -64, -128} {
		assert.Error(t, codec.ValidateChunkSize(size), "shareSize %d", size)
	}
	assert.NoError(t, codec.ValidateChunkSize(64))
}

func TestNewExtendedDataSquareRejectsZeroWidth(t *testing.T) {
	_, err := NewExtendedDataSquare(NewLeoRSCodec(), NewDefaultTree, 0, shareSize)
	assert.Error(t, err)
}
