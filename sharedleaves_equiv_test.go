package rsmt2d

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"math/rand"
	"testing"

	"github.com/celestiaorg/nmt"
	"github.com/stretchr/testify/require"
)

const testNsSize = 29

// sortedNamespacedShares returns k*k shares with namespaces increasing in
// row-major order, so every Q0 row and column is sorted.
func sortedNamespacedShares(rnd *rand.Rand, k, size int) [][]byte {
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

// nmtSharedLeaves is a SharedLeafRootComputer for erasuredNamespacedMerkleTree.
// It mirrors what a celestia-app wrapper implementation has to do.
type nmtSharedLeaves struct {
	squareSize uint64
	// hashNodeOnlyOrdering skips the leaf-by-leaf namespace ordering check.
	// It exists to show that the check is required.
	hashNodeOnlyOrdering bool
}

func (n nmtSharedLeaves) hasher() *nmt.NmtHasher {
	return nmt.NewNmtHasher(sha256.New(), testNsSize, true)
}

func (n nmtSharedLeaves) AppendLeafHash(dst []byte, r, c uint, share []byte) ([]byte, error) {
	if uint64(r) >= 2*n.squareSize || uint64(c) >= 2*n.squareSize {
		return nil, fmt.Errorf("cell (%d, %d) outside square", r, c)
	}
	if len(share) < testNsSize {
		return nil, errors.New("data is too short to contain namespace ID")
	}
	nd := make([]byte, testNsSize+len(share))
	copy(nd[testNsSize:], share)
	if uint64(r) < n.squareSize && uint64(c) < n.squareSize {
		copy(nd[:testNsSize], share[:testNsSize])
	} else {
		copy(nd[:testNsSize], bytes.Repeat([]byte{0xFF}, testNsSize))
	}
	h, err := n.hasher().HashLeaf(nd)
	if err != nil {
		return nil, err
	}
	return append(dst, h...), nil
}

func (n nmtSharedLeaves) LeafHashSize() int { return 2*testNsSize + sha256.Size }

func (n nmtSharedLeaves) RootFromLeafHashes(_ Axis, _ uint, hashes [][]byte) ([]byte, error) {
	if !n.hashNodeOnlyOrdering {
		// Push rejects a leaf whose namespace is below the previous one.
		// HashNode's range check is weaker: with IgnoreMaxNamespace it skips
		// leaves in the max namespace, so ordering is checked here.
		for i := 1; i < len(hashes); i++ {
			if bytes.Compare(hashes[i][:testNsSize], hashes[i-1][:testNsSize]) < 0 {
				return nil, fmt.Errorf("leaf %d is out of namespace order", i)
			}
		}
	}
	return nmtRoot(n.hasher(), hashes)
}

func nmtRoot(h *nmt.NmtHasher, hashes [][]byte) ([]byte, error) {
	switch len(hashes) {
	case 0:
		return h.EmptyRoot(), nil
	case 1:
		return hashes[0], nil
	}
	k := 1 << (bits.Len(uint(len(hashes))) - 1)
	if k == len(hashes) {
		k >>= 1
	}
	l, err := nmtRoot(h, hashes[:k])
	if err != nil {
		return nil, err
	}
	r, err := nmtRoot(h, hashes[k:])
	if err != nil {
		return nil, err
	}
	return h.HashNode(l, r)
}

func TestComputeRootsSharedLeavesMatchesNMT(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for _, k := range []int{1, 2, 4, 8, 16, 32, 64} {
		t.Run(fmt.Sprintf("k=%d", k), func(t *testing.T) {
			shares := sortedNamespacedShares(rnd, k, 512)
			ctor := newErasuredNamespacedMerkleTreeConstructor(uint64(k), nmt.NamespaceIDSize(testNsSize))
			regular, err := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), ctor)
			require.NoError(t, err)
			shared, err := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), ctor, nmtSharedLeaves{squareSize: uint64(k)})
			require.NoError(t, err)
			requireSameRoots(t, regular, shared)
		})
	}
}

// TestComputeRootsSharedLeavesNMTOrdering checks that the shared-leaf path
// rejects exactly what the regular NMT path rejects, including a row where a
// max-namespace leaf hides an ordering violation from HashNode.
func TestComputeRootsSharedLeavesNMTOrdering(t *testing.T) {
	const k = 4
	maxNs := bytes.Repeat([]byte{0xFF}, testNsSize)
	cases := map[string]func(shares [][]byte){
		"swapped": func(s [][]byte) { s[1], s[2] = s[2], s[1] },
		// Row 1 and column 1 both become n, MAX, n', n'' with n < n': out of
		// order, but MAX is a right child so HashNode ignores it.
		"max namespace in Q0": func(s [][]byte) { copy(s[k+1][:testNsSize], maxNs) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			shares := sortedNamespacedShares(rand.New(rand.NewSource(2)), k, 512)
			mutate(shares)
			ctor := newErasuredNamespacedMerkleTreeConstructor(k, nmt.NamespaceIDSize(testNsSize))

			regular, err := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), ctor)
			require.NoError(t, err)
			_, err = regular.RowRoots()
			require.Error(t, err)

			shared, err := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), ctor, nmtSharedLeaves{squareSize: k})
			require.NoError(t, err)
			_, err = shared.RowRoots()
			require.Error(t, err)
		})
	}

	// Without the leaf-by-leaf check the max-namespace case is accepted,
	// which would diverge from the regular path.
	shares := sortedNamespacedShares(rand.New(rand.NewSource(2)), k, 512)
	cases["max namespace in Q0"](shares)
	ctor := newErasuredNamespacedMerkleTreeConstructor(k, nmt.NamespaceIDSize(testNsSize))
	naive, err := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), ctor, nmtSharedLeaves{squareSize: k, hashNodeOnlyOrdering: true})
	require.NoError(t, err)
	_, err = naive.RowRoots()
	require.NoError(t, err, "HashNode alone does not catch this ordering violation")
}

// eraseCells removes cells from eds so that it needs repair.
func eraseCells(eds *ExtendedDataSquare, rnd *rand.Rand, n int) {
	w := int(eds.width)
	for i := 0; i < n; i++ {
		r, c := uint(rnd.Intn(w)), uint(rnd.Intn(w))
		eds.squareRow[r][c] = nil
		eds.squareCol[c][r] = nil
	}
	eds.resetRoots()
}

// TestSharedLeavesRepairMatchesRegular repairs the same partial square with
// and without a SharedLeafRootComputer and checks that the repaired shares,
// roots and byzantine errors are identical.
func TestSharedLeavesRepairMatchesRegular(t *testing.T) {
	const k = 8
	for seed := int64(0); seed < 8; seed++ {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			shares := sortedNamespacedShares(rand.New(rand.NewSource(seed)), k, 512)
			ctor := newErasuredNamespacedMerkleTreeConstructor(k, nmt.NamespaceIDSize(testNsSize))
			newShared := func() *ExtendedDataSquare {
				eds, err := ComputeExtendedDataSquareSharedLeaves(shares, NewLeoRSCodec(), ctor, nmtSharedLeaves{squareSize: k})
				require.NoError(t, err)
				return eds
			}
			ref := newShared()
			rowRoots, err := ref.RowRoots()
			require.NoError(t, err)
			colRoots, err := ref.ColRoots()
			require.NoError(t, err)

			regular, err := ComputeExtendedDataSquare(shares, NewLeoRSCodec(), ctor)
			require.NoError(t, err)
			shared := newShared()
			// Roots are computed (and cached) before cells are erased.
			_, err = shared.RowRoots()
			require.NoError(t, err)

			eraseCells(regular, rand.New(rand.NewSource(seed)), 2*k*k/3)
			eraseCells(shared, rand.New(rand.NewSource(seed)), 2*k*k/3)
			if seed%2 == 1 {
				// Corrupt the same present Q0 cell in both squares.
				for i := uint(0); i < k*k; i++ {
					r, c := i/k, i%k
					if regular.squareRow[r][c] != nil {
						bad := bytes.Clone(regular.squareRow[r][c])
						bad[len(bad)-1] ^= 0xFF
						regular.squareRow[r][c], regular.squareCol[c][r] = bad, bad
						shared.squareRow[r][c], shared.squareCol[c][r] = bad, bad
						// Leave column c incomplete: the pre-repair check runs
						// rows and columns concurrently, so a corrupt cell in a
						// complete row and column is reported on either axis.
						r2 := (r + 1) % (2 * k)
						regular.squareRow[r2][c], regular.squareCol[c][r2] = nil, nil
						shared.squareRow[r2][c], shared.squareCol[c][r2] = nil, nil
						break
					}
				}
			}

			errRegular := regular.Repair(rowRoots, colRoots)
			errShared := shared.Repair(rowRoots, colRoots)
			require.Equal(t, errRegular, errShared)
			if seed%2 == 1 {
				var byz *ErrByzantineData
				require.ErrorAs(t, errShared, &byz)
			} else {
				require.NoError(t, errShared)
			}
			require.Equal(t, regular.Flattened(), shared.Flattened())
			if errRegular == nil {
				requireSameRoots(t, regular, shared)
				requireSameRoots(t, ref, shared)
			}
		})
	}
}
