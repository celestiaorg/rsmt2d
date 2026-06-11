package rsmt2d

import (
	"crypto/sha256"
	"fmt"

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

// Tree wraps Merkle tree implementations to work with rsmt2d
type Tree interface {
	Push(data []byte) error
	Root() ([]byte, error)
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

const (
	// DefaultTreeName is the tree name of the default tree, a plain sha256
	// Merkle tree.
	DefaultTreeName = "default-tree"
	// NMTTreeName is the tree name of Celestia's erasured namespaced Merkle
	// tree.
	NMTTreeName = "nmt"
)

// treeConstructorByName returns the TreeConstructorFn for one of the built-in
// tree names. odsWidth is the width of the original (unextended) data square;
// it is required by trees whose shape depends on the square size and ignored
// by the others.
func treeConstructorByName(treeName string, odsWidth uint) (TreeConstructorFn, error) {
	switch treeName {
	case DefaultTreeName:
		return NewDefaultTree, nil
	case NMTTreeName:
		return nmtTreeConstructor(odsWidth), nil
	default:
		return nil, fmt.Errorf("unsupported tree name %q (supported: %q, %q)", treeName, DefaultTreeName, NMTTreeName)
	}
}
