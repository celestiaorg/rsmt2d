package rsmt2d

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/nmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTreeConstructorByName(t *testing.T) {
	t.Run("returns the default tree for DefaultTreeName", func(t *testing.T) {
		constructorFn, err := treeConstructorByName(DefaultTreeName, 2)
		require.NoError(t, err)

		got := constructorFn(Row, 0)
		require.NoError(t, got.Push([]byte("data")))
		gotRoot, err := got.Root()
		require.NoError(t, err)

		want := NewDefaultTree(Row, 0)
		require.NoError(t, want.Push([]byte("data")))
		wantRoot, err := want.Root()
		require.NoError(t, err)

		assert.Equal(t, wantRoot, gotRoot)
	})

	t.Run("returns the erasured NMT for NMTTreeName", func(t *testing.T) {
		const odsWidth = 2
		share := bytes.Repeat([]byte{1}, shareSize)

		constructorFn, err := treeConstructorByName(NMTTreeName, odsWidth)
		require.NoError(t, err)

		got := constructorFn(Row, 0)
		// Push a full extended row (2 * odsWidth shares).
		for i := 0; i < 2*odsWidth; i++ {
			require.NoError(t, got.Push(share))
		}
		gotRoot, err := got.Root()
		require.NoError(t, err)

		want := newErasuredNamespacedMerkleTreeConstructor(odsWidth,
			nmt.NamespaceIDSize(defaultNamespaceIDSize))(Row, 0)
		for i := 0; i < 2*odsWidth; i++ {
			require.NoError(t, want.Push(share))
		}
		wantRoot, err := want.Root()
		require.NoError(t, err)

		assert.Equal(t, wantRoot, gotRoot)
	})

	t.Run("returns an error for an unknown tree name", func(t *testing.T) {
		_, err := treeConstructorByName("unknown-tree", 2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported tree name")
	})
}
