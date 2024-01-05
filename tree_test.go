package rsmt2d

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterTree(t *testing.T) {
	treeName := "testing_register_tree"
	treeConstructorFn := sudoConstructorFn

	tests := []struct {
		name      string
		expectErr error
	}{
		{"register successfully", nil},
		{"register unsuccessfully", fmt.Errorf("%s already registered", treeName)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// By registering the function on the successful testcase first
			// the tree name will be registered already so we can check
			// the unsuccessful testcase
			err := RegisterTree(treeName, treeConstructorFn)
			if test.expectErr != nil {
				fmt.Println(err)
				require.Equal(t, test.expectErr, err)
			}

			treeFn, err := TreeFn(treeName)
			require.NoError(t, err)
			require.True(t, reflect.DeepEqual(reflect.ValueOf(treeFn), reflect.ValueOf(treeConstructorFn)))
		})
	}

	cleanUp(treeName)
}

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
		{
			"get successfully",
			treeName,
			func() {
				err := RegisterTree(treeName, treeConstructorFn)
				require.NoError(t, err)
			},
			nil,
		},
		{
			"get unregisted tree name",
			"unregistered_tree",
			func() {},
			fmt.Errorf("%s not registered yet", "unregistered_tree"),
		},
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

// TODO: When we handle all the breaking changes track in this PR: https://github.com/celestiaorg/rsmt2d/pull/278,
// should remove this test
func TestGetTreeNameFromConstructorFn(t *testing.T) {
	treeName := "testing_get_tree_name_tree"
	treeConstructorFn := sudoConstructorFn
	invalidTreeName := struct{}{}
	invalidCaseTreeName := "invalid_case_tree"
	invalidTreeConstructorFn := "invalid constructor fn"

	tests := []struct {
		name         string
		treeName     string
		treeFn       TreeConstructorFn
		malleate     func()
		expectGetKey bool
	}{
		{
			"get successfully",
			treeName,
			treeConstructorFn,
			func() {
				err := RegisterTree(treeName, treeConstructorFn)
				require.NoError(t, err)
			},
			true,
		},
		{
			"get unregisted tree name",
			"unregisted_tree_name",
			nil,
			func() {},
			false,
		},
		{
			"get invalid interface value",
			"",
			nil,
			func() {
				// Seems like this case has low probability of happening
				// since all register has been done through RegisterTree func
				// which have strict type check as argument.
				treeFns.Store(invalidCaseTreeName, invalidTreeConstructorFn)
			},
			false,
		},
		{
			"get invalid interface key",
			"",
			nil,
			func() {
				// Seems like this case has low probability of happening
				// since all register has been done through RegisterTree func
				// which have strict type check as argument.
				treeFns.Store(invalidTreeName, treeConstructorFn)
			},
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.malleate()

			key := getTreeNameFromConstructorFn(test.treeFn)
			if !test.expectGetKey {
				require.Equal(t, key, "")
			} else {
				require.Equal(t, test.treeName, key)
			}
		})

		cleanUp(test.treeName)
	}
}

// Avoid duplicate with default_tree treeConstructorFn
// registered during init
func sudoConstructorFn(_ Axis, _ uint) Tree {
	return &DefaultTree{}
}

func cleanUp(treeName string) {
	removeTreeFn(treeName)
}
