package rsmt2d

import "fmt"

// TreeConstructorFn creates a fresh Tree instance to be used as the Merkle tree
// inside of rsmt2d.
type TreeConstructorFn = func(axis Axis, index uint) Tree

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

// trees is a global map used for keeping track of registered tree constructors for JSON serialization
// The keys of this map should be kebab cased. E.g. "default-tree"
var trees = make(map[string]TreeConstructorFn)

func RegisterTree(treeName string, treeConstructor TreeConstructorFn) error {
	if trees[treeName] != nil {
		return fmt.Errorf("%s already registered", treeName)
	}
	trees[treeName] = treeConstructor

	return nil
}
