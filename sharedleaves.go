package rsmt2d

import (
	"golang.org/x/sync/errgroup"
)

// SharedLeafRootComputer is an optional, faster way to compute all row and
// column roots of an extended data square.
//
// In an EDS every cell is a leaf of exactly two trees (its row tree and its
// column tree). For tree schemes where the leaf hash of a cell does not depend
// on which axis it is pushed to (true for the celestia NMT wrapper: the
// namespace used for a cell depends only on whether the cell lies in Q0),
// hashing every cell once and reusing the leaf hash for both trees halves the
// leaf-hashing work while producing bit-identical roots.
//
// Implementations must be safe for concurrent use.
type SharedLeafRootComputer interface {
	// AppendLeafHash appends the leaf hash of share located at (rowIdx,
	// colIdx) of the EDS to dst and returns the extended slice. The result
	// must equal the leaf hash that the regular row tree and column tree
	// would compute for that cell.
	AppendLeafHash(dst []byte, rowIdx, colIdx uint, share []byte) ([]byte, error)
	// LeafHashSize returns the size in bytes of a leaf hash.
	LeafHashSize() int
	// RootFromLeafHashes computes the root of one axis (row or column) from
	// its ordered leaf hashes. It must perform the same validation (e.g.
	// namespace ordering) as the regular tree would.
	RootFromLeafHashes(axis Axis, axisIdx uint, leafHashes [][]byte) ([]byte, error)
}

// ComputeExtendedDataSquareSharedLeaves is ComputeExtendedDataSquare but
// computes row/column roots via rc (see SharedLeafRootComputer). The
// treeCreatorFn is still used for single-axis operations such as repair.
func ComputeExtendedDataSquareSharedLeaves(
	data [][]byte,
	codec Codec,
	treeCreatorFn TreeConstructorFn,
	rc SharedLeafRootComputer,
) (*ExtendedDataSquare, error) {
	eds, err := ComputeExtendedDataSquare(data, codec, treeCreatorFn)
	if err != nil {
		return nil, err
	}
	eds.sharedLeafRC = rc
	return eds, nil
}

func (ds *dataSquare) computeRootsSharedLeaves(rc SharedLeafRootComputer) error {
	w := ds.width
	hs := rc.LeafHashSize()
	buf := make([]byte, int(w*w)*hs)
	leaf := func(r, c uint) []byte {
		off := (int(r*w + c)) * hs
		return buf[off : off+hs : off+hs]
	}

	var g errgroup.Group
	if ds.parallelOps != 0 {
		g.SetLimit(ds.parallelOps)
	}
	// Phase 1: hash every cell once (one task per row).
	for r := uint(0); r < w; r++ {
		g.Go(func() error {
			row := ds.squareRow[r]
			for c := uint(0); c < w; c++ {
				if row[c] == nil {
					return errIncompleteSquare
				}
				if _, err := rc.AppendLeafHash(leaf(r, c)[:0], r, c, row[c]); err != nil {
					return err
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Phase 2: inner nodes for every row and every column.
	rowRoots := make([][]byte, w)
	colRoots := make([][]byte, w)
	for i := uint(0); i < w; i++ {
		g.Go(func() error {
			hashes := make([][]byte, w)
			for c := uint(0); c < w; c++ {
				hashes[c] = leaf(i, c)
			}
			root, err := rc.RootFromLeafHashes(Row, i, hashes)
			rowRoots[i] = root
			return err
		})
		g.Go(func() error {
			hashes := make([][]byte, w)
			for r := uint(0); r < w; r++ {
				hashes[r] = leaf(r, i)
			}
			root, err := rc.RootFromLeafHashes(Col, i, hashes)
			colRoots[i] = root
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	ds.rowRoots = rowRoots
	ds.colRoots = colRoots
	return nil
}
