package rsmt2d

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	mrand "math/rand"
	"testing"

	"github.com/celestiaorg/nmt"
	"github.com/stretchr/testify/require"
)

const testNsSize = 29

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

// sortedNamespacedShares returns k*k shares with namespaces increasing in
// row-major order, so every Q0 row and column is sorted.
func sortedNamespacedShares(rnd *mrand.Rand, k, size int) [][]byte {
	out := make([][]byte, k*k)
	for i := range out {
		out[i] = make([]byte, size)
		rnd.Read(out[i])
		ns := out[i][:testNsSize]
		clear(ns)
		binary.BigEndian.PutUint64(ns[testNsSize-8:], uint64(i))
	}
	return out
}

func requireSameRoots(t *testing.T, want, got *ExtendedDataSquare) {
	t.Helper()
	wr, err := want.RowRoots()
	require.NoError(t, err)
	wc, err := want.ColRoots()
	require.NoError(t, err)
	gr, err := got.RowRoots()
	require.NoError(t, err)
	gc, err := got.ColRoots()
	require.NoError(t, err)
	require.Equal(t, wr, gr)
	require.Equal(t, wc, gc)
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

// TestErasureExtendArenaLargeSquares compares the arena and allocating
// extension paths, shares and NMT roots, for the square sizes celestia-app
// uses. k >= 256 exercises the 16-bit leopard field.
func TestErasureExtendArenaLargeSquares(t *testing.T) {
	sizes := []int{64, 128, 256}
	if !testing.Short() && !raceEnabled {
		sizes = append(sizes, 512)
	}
	rnd := mrand.New(mrand.NewSource(3))
	for _, k := range sizes {
		t.Run(fmt.Sprintf("k=%d", k), func(t *testing.T) {
			shares := sortedNamespacedShares(rnd, k, 512)
			ctor := newErasuredNamespacedMerkleTreeConstructor(uint64(k), nmt.NamespaceIDSize(testNsSize))
			arena, err := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), ctor)
			require.NoError(t, err)
			plain, err := ComputeExtendedDataSquare(shares, plainCodec{NewLeoRSCodec()}, ctor)
			require.NoError(t, err)
			require.True(t, arena.Equals(plain))
			if k <= 256 && (k <= 128 || !raceEnabled) {
				requireSameRoots(t, plain, arena)
			}
		})
	}
}

var errTestEncodeInto = errors.New("test encodeInto failure")

type failingIntoCodec struct{ *LeoRSCodec }

func (failingIntoCodec) encodeInto(_, _ [][]byte) error { return errTestEncodeInto }

func TestEncodeIntoErrors(t *testing.T) {
	shares := randomShares(t, 2*2, 64)
	eds, err := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), NewDefaultTree)
	require.NoError(t, err)
	arena := &parityArena{buf: make([]byte, 3*2*2*64), k: 2, shareSize: 64}
	_, err = eds.encode(failingIntoCodec{NewLeoRSCodec()}, shares[:2], arena, 0)
	require.ErrorIs(t, err, errTestEncodeInto)

	require.Error(t, NewLeoRSCodec().encodeInto(nil, nil), "empty input must be rejected")
}

func BenchmarkExtensionArena(b *testing.B) {
	for _, k := range []int{32, 64, 128} {
		shares := make([][]byte, k*k)
		for i := range shares {
			shares[i] = make([]byte, 512)
			_, _ = rand.Read(shares[i])
		}
		for _, c := range []struct {
			name  string
			codec Codec
		}{{"plain", plainCodec{NewLeoRSCodec()}}, {"arena", NewLeoRSCodec()}} {
			codec := c.codec
			b.Run(fmt.Sprintf("%s/k=%d", c.name, k), func(b *testing.B) {
				for b.Loop() {
					_, _ = ComputeExtendedDataSquare(shares, codec, NewDefaultTree)
				}
			})
		}
	}
}
