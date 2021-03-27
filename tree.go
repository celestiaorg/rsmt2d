package rsmt2d

import (
	"crypto/sha256"
	"fmt"

	"github.com/lazyledger/merkletree"
)

// TreeConstructorFn creates a fresh Tree instance to be used as the Merkle inside of rsmt2d.
type TreeConstructorFn = func() Tree

// SquareIndex contains all information needed to identify the cell that is being
// pushed
type SquareIndex struct {
	Axis, Cell uint
}

// Tree wraps merkle tree implementations to work with rsmt2d
type Tree interface {
	Push(data []byte, idx SquareIndex)
	// TODO(ismail): is this general enough?
	Prove(idx int) (merkleRoot []byte, proofSet [][]byte, proofIndex uint64, numLeaves uint64)
	Root() []byte
}

var _ Tree = &DefaultTree{}

type DefaultTree struct {
	*merkletree.Tree
	leaves [][]byte
	root   []byte
}

func NewDefaultTree() Tree {
	return &DefaultTree{
		Tree:   merkletree.New(sha256.New()),
		leaves: make([][]byte, 0, 128),
	}
}

func (d *DefaultTree) Push(data []byte, _idx SquareIndex) {
	// ignore the idx, as this implementation doesn't need that info
	d.leaves = append(d.leaves, data)
}

func (d *DefaultTree) Prove(idx int) (merkleRoot []byte, proofSet [][]byte, proofIndex uint64, numLeaves uint64) {
	if err := d.Tree.SetIndex(uint64(idx)); err != nil {
		panic(fmt.Sprintf("don't call prove on a already used tree: %v", err))
	}
	for _, l := range d.leaves {
		d.Tree.Push(l)
	}
	return d.Tree.Prove()
}

func (d *DefaultTree) Root() []byte {
	if d.root == nil {
		for _, l := range d.leaves {
			d.Tree.Push(l)
		}
		d.root = d.Tree.Root()
	}
	return d.root
}
