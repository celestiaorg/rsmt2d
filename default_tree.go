package rsmt2d

import (
	"crypto/sha256"
	"fmt"

	"github.com/celestiaorg/merkletree"
)

var DefaultTreeName = "default-tree"

func init() {
	err := RegisterTree(DefaultTreeName, NewDefaultTree)
	if err != nil {
		panic(fmt.Sprintf("%s already registered", DefaultTreeName))
	}
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
