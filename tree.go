package rsmt2d

import (
	"crypto/sha256"
	"fmt"

	"github.com/NebulousLabs/merkletree"
)

var DefaultTreeCreator = &defaultTreeCreator{}

type Tree interface {
	Push(data []byte)
	// TODO(ismail): is this general enough?
	Prove(idx int) (merkleRoot []byte, proofSet [][]byte, proofIndex uint64, numLeaves uint64)
	Root() []byte
}

type TreeCreator interface {
	New() Tree
}

var _ TreeCreator = &defaultTreeCreator{}
var _ Tree = &DefaultTree{}

type defaultTreeCreator struct{}
type DefaultTree struct {
	*merkletree.Tree
	leaves [][]byte
	root   []byte
}

func (d *defaultTreeCreator) New() Tree {
	return &DefaultTree{
		Tree:   merkletree.New(sha256.New()),
		leaves: make([][]byte, 0, 128),
	}
}

func (d *DefaultTree) Push(data []byte) {
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
