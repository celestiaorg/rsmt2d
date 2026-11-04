package rsmt2d

import (
	"crypto/sha256"

	"github.com/celestiaorg/merkletree"
)

// TreeConstructorFn creates a fresh Tree instance to be used as the Merkle tree
// inside of rsmt2d.
type TreeConstructorFn = func(axis Axis, index uint) Tree

type BufferedTreeConstructor interface {
	NewConstructor(squareSize uint) TreeConstructorFn
	TreeCount() int
}

// SquareIndex contains all information needed to identify the cell that is being
// pushed
type SquareIndex struct {
	Axis, Cell uint
}

// Tree wraps Merkle tree implementations to work with rsmt2d.
//
// rsmt2d calls Root exactly once on every Tree returned by a TreeConstructorFn,
// including after a Push returns an error, in which case the result of Root is
// discarded. Implementations that hold pooled resources (e.g. a tree obtained
// from a BufferedTreeConstructor) should release them in Root and must not
// release them when Push fails.
type Tree interface {
	Push(data []byte) error
	Root() ([]byte, error)
}

// releaseTree must be called on a tree whose root will not be computed, e.g.
// because a Push failed, to uphold the Tree contract that Root is called once
// per tree. Without this call a pooled tree would be leaked and a bounded pool
// could eventually block forever.
func releaseTree(tree Tree) {
	_, _ = tree.Root()
}

var _ Tree = &DefaultTree{}

type DefaultTree struct {
	*merkletree.Tree
	leaves [][]byte
	root   []byte
}

func NewDefaultTree(_ Axis, _ uint) Tree {
	return &DefaultTree{
		Tree:   merkletree.New(sha256.New()),
		leaves: make([][]byte, 0, 128),
	}
}

func (d *DefaultTree) Push(data []byte) error {
	// ignore the idx, as this implementation doesn't need that info
	d.leaves = append(d.leaves, data)
	return nil
}

func (d *DefaultTree) Root() ([]byte, error) {
	if d.root == nil {
		for _, l := range d.leaves {
			d.Tree.Push(l)
		}
		d.root = d.Tree.Root()
	}
	return d.root, nil
}
