package rsmt2d

import (
	"crypto/rand"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

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

// plainCodec hides LeoRSCodec's encodeInto so that the extension falls back to
// the allocating Encode path.
type plainCodec struct{ Codec }

// TestErasureExtendArenaMatchesAllocatingPath checks that the arena path gives
// byte-identical shares. k = 256 uses the 16-bit leopard field.
func TestErasureExtendArenaMatchesAllocatingPath(t *testing.T) {
	for _, k := range []int{1, 2, 4, 16, 64, 256} {
		t.Run(fmt.Sprintf("k=%d", k), func(t *testing.T) {
			shares := randomShares(t, k*k, 512)
			arena, err := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), NewDefaultTree)
			require.NoError(t, err)
			plain, err := ComputeExtendedDataSquare(shares, plainCodec{NewLeoRSCodec()}, NewDefaultTree)
			require.NoError(t, err)
			require.True(t, arena.Equals(plain))
		})
	}
}

// TestErasureExtendArenaIncompleteSquare checks that an incomplete original
// square fails like the allocating path without allocating the arena.
func TestErasureExtendArenaIncompleteSquare(t *testing.T) {
	const k, shareSize = 32, 64 * 1024
	data := make([][]byte, k*k)
	data[0] = make([]byte, shareSize)

	_, wantErr := ComputeExtendedDataSquare(data, plainCodec{NewLeoRSCodec()}, NewDefaultTree)
	require.Error(t, wantErr)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := ComputeExtendedDataSquare(data, NewLeoRSCodec(), NewDefaultTree)
	runtime.ReadMemStats(&after)
	require.Equal(t, wantErr, err)
	// The arena would be 3*k*k*shareSize = 192 MiB.
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(32<<20))
}

func TestEncodeIntoErrors(t *testing.T) {
	require.Error(t, NewLeoRSCodec().encodeInto(nil, nil), "empty input must be rejected")
}

// wrappedCodec embeds *LeoRSCodec and overrides Encode.
type wrappedCodec struct {
	*LeoRSCodec
	calls *atomic.Int64
}

func (w wrappedCodec) Encode(data [][]byte) ([][]byte, error) {
	w.calls.Add(1)
	return w.LeoRSCodec.Encode(data)
}

// TestErasureExtendUsesOverriddenEncode checks that the arena is only used for
// *LeoRSCodec itself, not for types that embed it and override Encode.
func TestErasureExtendUsesOverriddenEncode(t *testing.T) {
	const k = 4
	codec := wrappedCodec{LeoRSCodec: NewLeoRSCodec(), calls: new(atomic.Int64)}
	_, err := ComputeExtendedDataSquare(randomShares(t, k*k, 512), codec, NewDefaultTree)
	require.NoError(t, err)
	require.EqualValues(t, 3*k, codec.calls.Load())
}
