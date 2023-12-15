package rsmt2d

import (
	"fmt"
	"reflect"
	"sync"
)

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

// treeFns is a global map used for keeping track of registered tree constructors for JSON serialization
// The keys of this map should be kebab cased. E.g. "default-tree"
var treeFns = sync.Map{}

// RegisterTree must be called in the init function
func RegisterTree(treeName string, treeConstructor TreeConstructorFn) error {
	if _, ok := treeFns.Load(treeName); ok {
		return fmt.Errorf("%s already registered", treeName)
	}

	treeFns.Store(treeName, treeConstructor)

	return nil
}

// TreeFn get tree constructor function by tree name from the global map registry
func TreeFn(treeName string) (TreeConstructorFn, error) {
	var treeFn TreeConstructorFn
	v, ok := treeFns.Load(treeName)
	if !ok {
		return nil, fmt.Errorf("%s not registered yet", treeName)
	}
	treeFn, ok = v.(TreeConstructorFn)
	if !ok {
		return nil, fmt.Errorf("key %s has invalid interface", treeName)
	}

	return treeFn, nil
}

// Get the tree name by the tree constructor function from the global map registry
func getTreeNameFromConstructorFn(treeConstructor TreeConstructorFn) string {
	key := ""
	treeFns.Range(func(k, v interface{}) bool {
		keyString, ok := k.(string)
		if !ok {
			return false
		}
		treeFn, ok := v.(TreeConstructorFn)
		if !ok {
			return false
		}

		if reflect.DeepEqual(reflect.ValueOf(treeFn), reflect.ValueOf(treeConstructor)) {
			key = keyString
			return false
		}

		return true
	})

	return key
}
