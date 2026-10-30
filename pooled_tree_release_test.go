package rsmt2d

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/nmt"
	"github.com/stretchr/testify/require"
)

// TestPooledTreeReleasedOnError verifies that a tree acquired from a bounded
// pool is returned to the pool when root computation fails.
func TestPooledTreeReleasedOnError(t *testing.T) {
	share := func(namespace byte) []byte { return bytes.Repeat([]byte{namespace}, shareSize) }

	const poolSize = 4
	newPool := func() *treePool {
		return newTreePool(2, poolSize, nmt.NamespaceIDSize(defaultNamespaceIDSize))
	}

	testCases := []struct {
		name    string
		rootErr func(pool *treePool) error
	}{
		{
			name: "getRowRoot incomplete row",
			rootErr: func(pool *treePool) error {
				ds, err := newDataSquare([][]byte{share(1), nil, share(3), share(4)}, pool.NewConstructor(2), shareSize)
				require.NoError(t, err)
				_, err = ds.getRowRoot(0)
				return err
			},
		},
		{
			name: "getColRoot incomplete column",
			rootErr: func(pool *treePool) error {
				ds, err := newDataSquare([][]byte{nil, share(2), share(3), share(4)}, pool.NewConstructor(2), shareSize)
				require.NoError(t, err)
				_, err = ds.getColRoot(0)
				return err
			},
		},
		{
			name: "getRowRoot out of order namespaces",
			rootErr: func(pool *treePool) error {
				ds, err := newDataSquare([][]byte{share(2), share(1), share(3), share(4)}, pool.NewConstructor(2), shareSize)
				require.NoError(t, err)
				_, err = ds.getRowRoot(0)
				return err
			},
		},
		{
			name: "getColRoot out of order namespaces",
			rootErr: func(pool *treePool) error {
				ds, err := newDataSquare([][]byte{share(2), share(3), share(1), share(4)}, pool.NewConstructor(2), shareSize)
				require.NoError(t, err)
				_, err = ds.getColRoot(0)
				return err
			},
		},
		{
			name: "computeSharesRoot out of order namespaces",
			rootErr: func(pool *treePool) error {
				eds := &ExtendedDataSquare{dataSquare: &dataSquare{createTreeFn: pool.NewConstructor(2)}}
				_, err := eds.computeSharesRoot([][]byte{share(2), share(1)}, Row, 0)
				return err
			},
		},
		{
			name: "computeSharesRootWithRebuiltShare out of order rebuilt share",
			rootErr: func(pool *treePool) error {
				eds := &ExtendedDataSquare{dataSquare: &dataSquare{createTreeFn: pool.NewConstructor(2)}}
				_, err := eds.computeSharesRootWithRebuiltShare([][]byte{share(2), nil}, Row, 0, 1, share(1))
				return err
			},
		},
		{
			name: "computeSharesRootWithRebuiltShare out of order trailing share",
			rootErr: func(pool *treePool) error {
				eds := &ExtendedDataSquare{dataSquare: &dataSquare{createTreeFn: pool.NewConstructor(2)}}
				_, err := eds.computeSharesRootWithRebuiltShare([][]byte{nil, share(1)}, Row, 0, 0, share(2))
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pool := newPool()
			// Repeat more times than the pool size: a leak would block forever.
			for range 2 * poolSize {
				require.Error(t, tc.rootErr(pool))
				require.Len(t, pool.availableNMTs, poolSize)
			}
		})
	}
}
