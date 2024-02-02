package rsmt2d

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterTree tests the RegisterTree function for adding
// a tree constructor function for a given tree name into treeFns
// global map.
func TestRegisterTree(t *testing.T) {
	treeName := "testing_register_tree"
	treeConstructorFn := sudoConstructorFn

	tests := []struct {
		name      string
		expectErr error
	}{
		// The tree has not been registered yet in the treeFns global map.
		{"register successfully", nil},
		// The tree has already been registered in the treeFns global map.
		{"register unsuccessfully", fmt.Errorf("%s already registered", treeName)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := RegisterTree(treeName, treeConstructorFn)
			if test.expectErr != nil {
				require.Equal(t, test.expectErr, err)
			}

			treeFn, err := TreeFn(treeName)
			require.NoError(t, err)
			assert.True(t, reflect.DeepEqual(reflect.ValueOf(treeFn), reflect.ValueOf(treeConstructorFn)))
		})
	}

	cleanUp(treeName)
}

// TestTreeFn test the TreeFn function which fetches the
// tree constructor function from the treeFns golbal map.
func TestTreeFn(t *testing.T) {
	treeName := "testing_treeFn_tree"
	treeConstructorFn := sudoConstructorFn
	invalidCaseTreeName := "testing_invalid_register_tree"
	invalidTreeConstructorFn := "invalid constructor fn"

	tests := []struct {
		name      string
		treeName  string
		malleate  func()
		expectErr error
	}{
		// The tree constructor function is successfully fetched
		// from the global map.
		{
			"get successfully",
			treeName,
			func() {
				err := RegisterTree(treeName, treeConstructorFn)
				require.NoError(t, err)
			},
			nil,
		},
		// Unable to fetch the tree constructor function for an
		// unregisted tree name.
		{
			"get unregisted tree name",
			"unregistered_tree",
			func() {},
			fmt.Errorf("%s not registered yet", "unregistered_tree"),
		},
		// Value returned from the global map is an invalid value that
		// cannot be type asserted into TreeConstructorFn type.
		{
			"get invalid interface value",
			invalidCaseTreeName,
			func() {
				// Seems like this case has low probability of happening
				// since all register has been done through RegisterTree func
				// which have strict type check as argument.
				treeFns.Store(invalidCaseTreeName, invalidTreeConstructorFn)
			},
			fmt.Errorf("key %s has invalid interface", invalidCaseTreeName),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.malleate()

			treeFn, err := TreeFn(test.treeName)
			if test.expectErr != nil {
				require.Equal(t, test.expectErr, err)
			} else {
				require.NoError(t, err)
				require.True(t, reflect.DeepEqual(reflect.ValueOf(treeFn), reflect.ValueOf(treeConstructorFn)))
			}
		})

		cleanUp(test.treeName)
	}
}

// Avoid duplicate with default_tree treeConstructorFn
// registered during init.
func sudoConstructorFn(_ Axis, _ uint) Tree {
	return &DefaultTree{}
}

// Clear tested tree constructor function in the global map.
func cleanUp(treeName string) {
	removeTreeFn(treeName)
}
