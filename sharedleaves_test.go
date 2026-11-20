package rsmt2d

import (
	"bytes"
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

func randomShares(t *testing.T, n, size int) [][]byte {
	t.Helper()
	out := make([][]byte, n)
	for i := range out {
		out[i] = make([]byte, size)
		_, err := rand.Read(out[i])
		require.NoError(t, err)
	}
	return out
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

// plainCodec hides LeoRSCodec's encodeInto so that the extension falls back to
// the allocating Encode path.
type plainCodec struct{ Codec }

func TestErasureExtendArenaMatchesAllocatingPath(t *testing.T) {
	for _, k := range []int{1, 2, 4, 16, 32} {
		t.Run(fmt.Sprintf("k=%d", k), func(t *testing.T) {
			shares := randomShares(t, k*k, 64)
			arena, err := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), NewDefaultTree)
			require.NoError(t, err)
			plain, err := ComputeExtendedDataSquare(shares, plainCodec{NewLeoRSCodec()}, NewDefaultTree)
			require.NoError(t, err)
			a, p := arena.Flattened(), plain.Flattened()
			require.Equal(t, len(p), len(a))
			for i := range p {
				require.True(t, bytes.Equal(p[i], a[i]), "share %d differs", i)
			}
		})
	}
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
