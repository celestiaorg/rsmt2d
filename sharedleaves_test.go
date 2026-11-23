package rsmt2d

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// rfcTree is a minimal RFC 6962 style Merkle tree used to check that
// computeRootsSharedLeaves produces exactly the same roots as the regular
// per-axis tree path.
type rfcTree struct{ leaves [][]byte }

func rfcLeafHash(data []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0})
	h.Write(data)
	return h.Sum(nil)
}

func rfcRootFromLeafHashes(hashes [][]byte) []byte {
	switch len(hashes) {
	case 0:
		s := sha256.Sum256(nil)
		return s[:]
	case 1:
		return hashes[0]
	}
	k := 1
	for k*2 < len(hashes) {
		k *= 2
	}
	h := sha256.New()
	h.Write([]byte{1})
	h.Write(rfcRootFromLeafHashes(hashes[:k]))
	h.Write(rfcRootFromLeafHashes(hashes[k:]))
	return h.Sum(nil)
}

func newRFCTree(Axis, uint) Tree { return &rfcTree{} }

func (t *rfcTree) Push(data []byte) error {
	t.leaves = append(t.leaves, append([]byte(nil), data...))
	return nil
}

func (t *rfcTree) Root() ([]byte, error) {
	hashes := make([][]byte, len(t.leaves))
	for i, l := range t.leaves {
		hashes[i] = rfcLeafHash(l)
	}
	return rfcRootFromLeafHashes(hashes), nil
}

type rfcSharedLeaves struct{}

func (rfcSharedLeaves) AppendLeafHash(dst []byte, _, _ uint, share []byte) ([]byte, error) {
	return append(dst, rfcLeafHash(share)...), nil
}

func (rfcSharedLeaves) LeafHashSize() int { return sha256.Size }

func (rfcSharedLeaves) RootFromLeafHashes(_ Axis, _ uint, leafHashes [][]byte) ([]byte, error) {
	return rfcRootFromLeafHashes(leafHashes), nil
}

func TestComputeRootsSharedLeavesMatchesRegularPath(t *testing.T) {
	for _, k := range []int{1, 2, 4, 8, 16, 32} {
		t.Run(fmt.Sprintf("k=%d", k), func(t *testing.T) {
			shares := randomShares(t, k*k, 64)

			regular, err := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), newRFCTree)
			require.NoError(t, err)
			shared, err := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), newRFCTree, rfcSharedLeaves{})
			require.NoError(t, err)

			wantRows, err := regular.RowRoots()
			require.NoError(t, err)
			wantCols, err := regular.ColRoots()
			require.NoError(t, err)
			gotRows, err := shared.RowRoots()
			require.NoError(t, err)
			gotCols, err := shared.ColRoots()
			require.NoError(t, err)

			require.Equal(t, wantRows, gotRows)
			require.Equal(t, wantCols, gotCols)
		})
	}
}

func TestComputeRootsSharedLeavesRespectsParallelOps(t *testing.T) {
	shares := randomShares(t, 16*16, 64)
	regular, err := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), newRFCTree)
	require.NoError(t, err)
	shared, err := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), newRFCTree, rfcSharedLeaves{})
	require.NoError(t, err)
	shared.setParallelOps(1)
	want, err := regular.RowRoots()
	require.NoError(t, err)
	got, err := shared.RowRoots()
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func BenchmarkRootsSharedLeaves(b *testing.B) {
	for _, k := range []int{32, 64, 128} {
		shares := make([][]byte, k*k)
		for i := range shares {
			shares[i] = make([]byte, 512)
			_, _ = rand.Read(shares[i])
		}
		b.Run(fmt.Sprintf("regular/k=%d", k), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				eds, _ := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), newRFCTree)
				_, _ = eds.RowRoots()
				_, _ = eds.ColRoots()
			}
		})
		b.Run(fmt.Sprintf("shared/k=%d", k), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				eds, _ := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), newRFCTree, rfcSharedLeaves{})
				_, _ = eds.RowRoots()
				_, _ = eds.ColRoots()
			}
		})
	}
}

type failingSharedLeaves struct {
	rfcSharedLeaves
	failLeaf, failRoot bool
}

var errTestSharedLeaves = fmt.Errorf("test shared leaves failure")

func (f failingSharedLeaves) AppendLeafHash(dst []byte, r, c uint, share []byte) ([]byte, error) {
	if f.failLeaf {
		return nil, errTestSharedLeaves
	}
	return f.rfcSharedLeaves.AppendLeafHash(dst, r, c, share)
}

func (f failingSharedLeaves) RootFromLeafHashes(axis Axis, i uint, hashes [][]byte) ([]byte, error) {
	if f.failRoot {
		return nil, errTestSharedLeaves
	}
	return f.rfcSharedLeaves.RootFromLeafHashes(axis, i, hashes)
}

func TestComputeRootsSharedLeavesErrors(t *testing.T) {
	shares := randomShares(t, 4*4, 64)

	_, err := ComputeExtendedDataSquareSharedLeaves(shares[:3], NewLeoRSCodec(), newRFCTree, rfcSharedLeaves{})
	require.Error(t, err, "non-square input must be rejected")

	for _, rc := range []SharedLeafRootComputer{
		failingSharedLeaves{failLeaf: true},
		failingSharedLeaves{failRoot: true},
	} {
		eds, err := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), newRFCTree, rc)
		require.NoError(t, err)
		_, err = eds.RowRoots()
		require.ErrorIs(t, err, errTestSharedLeaves)
	}

	eds, err := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), newRFCTree, rfcSharedLeaves{})
	require.NoError(t, err)
	eds.squareRow[1][2] = nil
	require.ErrorIs(t, eds.computeRootsSharedLeaves(rfcSharedLeaves{}), errIncompleteSquare)
}

// shortLeafHashes returns leaf hashes one byte shorter than LeafHashSize.
type shortLeafHashes struct{ rfcSharedLeaves }

func (shortLeafHashes) AppendLeafHash(dst []byte, _, _ uint, share []byte) ([]byte, error) {
	return append(dst, rfcLeafHash(share)[1:]...), nil
}

// freshLeafHashes ignores dst and returns a newly allocated leaf hash.
type freshLeafHashes struct{ rfcSharedLeaves }

func (freshLeafHashes) AppendLeafHash(_ []byte, _, _ uint, share []byte) ([]byte, error) {
	return rfcLeafHash(share), nil
}

// longLeafHashes returns leaf hashes one byte longer than LeafHashSize.
type longLeafHashes struct{ rfcSharedLeaves }

func (longLeafHashes) AppendLeafHash(dst []byte, _, _ uint, share []byte) ([]byte, error) {
	return append(append(dst, rfcLeafHash(share)...), 0), nil
}

// zeroSizeLeafHashes reports a leaf hash size of zero.
type zeroSizeLeafHashes struct{ rfcSharedLeaves }

func (zeroSizeLeafHashes) LeafHashSize() int { return 0 }

func TestComputeRootsSharedLeavesChecksLeafHashes(t *testing.T) {
	shares := randomShares(t, 4*4, 64)
	regular, err := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), newRFCTree)
	require.NoError(t, err)
	want, err := regular.RowRoots()
	require.NoError(t, err)

	// A leaf hash that is not written into dst must still be used.
	eds, err := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), newRFCTree, freshLeafHashes{})
	require.NoError(t, err)
	got, err := eds.RowRoots()
	require.NoError(t, err)
	require.Equal(t, want, got)

	// A leaf hash of the wrong size must be rejected, not silently used.
	for name, rc := range map[string]SharedLeafRootComputer{
		"short": shortLeafHashes{},
		"long":  longLeafHashes{},
		"zero":  zeroSizeLeafHashes{},
	} {
		eds, err := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), newRFCTree, rc)
		require.NoError(t, err)
		_, err = eds.RowRoots()
		require.Error(t, err, name)
	}
}
