package rsmt2d

// The contents of this file have been adapted from the source file available at https://github.com/celestiaorg/celestia-app/blob/bab6c0d0befe677ab8c2f4b83561c08affc7203e/pkg/wrapper/nmt_wrapper.go,
// solely for the purpose of testing rsmt2d expected behavior when integrated with a NamespaceMerkleTree.
// Please note that this file has undergone several modifications and may not match the original file exactly.

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

type bytePool struct {
	pool *sync.Pool
}

func newBytePool(capacity int) *bytePool {
	return &bytePool{
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, capacity)
			},
		},
	}
}

func (bp *bytePool) getOrAlloc(requiredLen int) []byte {
	if pooled := bp.pool.Get(); pooled != nil {
		b := pooled.([]byte)
		if cap(b) >= requiredLen {
			return b[:requiredLen]
		}
	}
	return make([]byte, requiredLen)
}

func (bp *bytePool) put(b []byte) {
	bp.pool.Put(b)
}

type treeFactory struct {
	squareSize    uint64
	opts          []nmt.Option
	treePool      *fixedTreePool
	byteSlicePool *bytePool
	shareSize     int
	namespaceSize int
}

func newTreeFactory(squareSize uint64, poolSize int, shareSize int, namespaceSize int, opts ...nmt.Option) *treeFactory {
	return &treeFactory{
		squareSize:    squareSize,
		opts:          opts,
		shareSize:     shareSize,
		namespaceSize: namespaceSize,
		treePool:      newFixedTreePool(poolSize, squareSize, opts),
		byteSlicePool: newBytePool(namespaceSize + shareSize),
	}
}

func (f *treeFactory) NewConstructor() TreeConstructorFn {
	return func(_ Axis, axisIndex uint) Tree {
		tree := f.treePool.acquire()
		tree.axisIndex = uint64(axisIndex)
		tree.shareIndex = 0
		tree.byteSlicePool = f.byteSlicePool

		leaves := tree.tree.Reset()
		for _, leaf := range leaves {
			if leaf != nil {
				f.byteSlicePool.put(leaf)
			}
		}

		return tree
	}
}

type fixedTreePool struct {
	availableNMTs chan *erasuredNamespacedMerkleTree
	opts          []nmt.Option
	squareSize    uint64
}

func newFixedTreePool(size int, squareSize uint64, opts []nmt.Option) *fixedTreePool {
	pool := &fixedTreePool{
		availableNMTs: make(chan *erasuredNamespacedMerkleTree, size),
		opts:          opts,
		squareSize:    squareSize,
	}

	for i := 0; i < size; i++ {
		tree := newErasuredNamespacedMerkleTree(squareSize, 0, opts...)
		treePtr := &tree
		treePtr.pool = pool
		pool.availableNMTs <- treePtr
	}

	return pool
}

func (p *fixedTreePool) acquire() *erasuredNamespacedMerkleTree {
	return <-p.availableNMTs
}

func (p *fixedTreePool) release(tree *erasuredNamespacedMerkleTree) {
	p.availableNMTs <- tree
}

// Fulfills the Tree interface and TreeConstructorFn function
var (
	_ Tree = &erasuredNamespacedMerkleTree{}
)

// erasuredNamespacedMerkleTree wraps NamespaceMerkleTree to conform to the
// Tree interface while also providing the correct namespaces to the
// underlying NamespaceMerkleTree. It does this by adding the already included
// namespace to the first half of the tree, and then uses the parity namespace
// ID for each share pushed to the second half of the tree. This allows for the
// namespaces to be included in the erasure data, while also keeping the nmt
// library sufficiently general
type erasuredNamespacedMerkleTree struct {
	squareSize uint64 // note: this refers to the width of the original square before erasure-coded
	options    []nmt.Option
	tree       nmtTree
	// axisIndex is the index of the axis (row or column) that this tree is on. This is passed
	// by rsmt2d and used to help determine which quadrant each leaf belongs to.
	axisIndex uint64
	// shareIndex is the index of the share in a row or column that is being
	// pushed to the tree. It is expected to be in the range: 0 <= shareIndex
	// 2*squareSize. shareIndex is used to help determine which quadrant each
	// leaf belongs to, along with keeping track of how many leaves have been
	// added to the tree so far.
	shareIndex      uint64
	namespaceSize   int
	pool            *fixedTreePool
	byteSlicePool   *bytePool
	parityNamespace []byte // Pre-allocated parity namespace bytes
}

// nmtTree is an interface that wraps the methods of the underlying
// NamespaceMerkleTree that are used by erasuredNamespacedMerkleTree. This
// interface is mainly used for testing. It is not recommended to use this
// interface by implementing a different implementation.
type nmtTree interface {
	Root() ([]byte, error)
	FastRoot() ([]byte, error)
	Push(namespacedData namespace.PrefixedData) error
	Reset() [][]byte
}

// newErasuredNamespacedMerkleTree creates a new erasuredNamespacedMerkleTree
// with an underlying NMT of namespace size `NamespaceSize` and with
// `ignoreMaxNamespace=true`. axisIndex is the index of the row or column that
// this tree is committing to. squareSize must be greater than zero.
func newErasuredNamespacedMerkleTree(squareSize uint64, axisIndex uint,
	options ...nmt.Option,
) erasuredNamespacedMerkleTree {
	if squareSize == 0 {
		panic("cannot create a erasuredNamespacedMerkleTree of squareSize == 0")
	}
	// read the options to extract the namespace size, and use it to construct erasuredNamespacedMerkleTree
	opts := &nmt.Options{}
	for _, setter := range options {
		setter(opts)
	}
	options = append(options, nmt.IgnoreMaxNamespace(true))
	tree := nmt.New(sha256.New(), options...)

	// Pre-allocate parity namespace and buffer to avoid repeated allocations
	namespaceSize := int(opts.NamespaceIDSize)
	parityNamespace := bytes.Repeat([]byte{0xFF}, namespaceSize)

	return erasuredNamespacedMerkleTree{
		squareSize:      squareSize,
		namespaceSize:   namespaceSize,
		options:         options,
		tree:            tree,
		axisIndex:       uint64(axisIndex),
		shareIndex:      0,
		pool:            nil,
		parityNamespace: parityNamespace,
	}
}

type constructor struct {
	squareSize uint64
	opts       []nmt.Option
}

// newErasuredNamespacedMerkleTreeConstructor creates a tree constructor function as required by rsmt2d to
// calculate the data root. It creates that tree using the
// erasuredNamespacedMerkleTree with a bounded pool of maxSize elements.
func newErasuredNamespacedMerkleTreeConstructor(squareSize uint64, opts ...nmt.Option) TreeConstructorFn {
	c := constructor{
		squareSize: squareSize,
		opts:       opts,
	}
	return c.NewTree
}

// NewTree creates a new Tree using the
// erasuredNamespacedMerkleTree with predefined square size and
// nmt.Options
func (c constructor) NewTree(_ Axis, axisIndex uint) Tree {
	tree := newErasuredNamespacedMerkleTree(c.squareSize, axisIndex, c.opts...)
	return &tree
}

// Release implements the Releasable interface for erasuredNamespacedMerkleTree
func (t *erasuredNamespacedMerkleTree) Release() {
	if t.pool != nil {
		t.pool.release(t)
	}
}

// Push adds the provided data to the underlying NamespaceMerkleTree, and
// automatically uses the first erasuredNamespacedMerkleTree.namespaceSize number of bytes as the
// namespace unless the data pushed to the second half of the tree. Fulfills the
// rsmt2d.Tree interface. NOTE: panics if an error is encountered while pushing or
// if the tree size is exceeded.
func (w *erasuredNamespacedMerkleTree) Push(data []byte) error {
	if w.axisIndex+1 > 2*w.squareSize || w.shareIndex+1 > 2*w.squareSize {
		return fmt.Errorf("pushed past predetermined square size: boundary at %d index at %d %d", 2*w.squareSize, w.axisIndex, w.shareIndex)
	}
	if len(data) < w.namespaceSize {
		return fmt.Errorf("data is too short to contain namespace ID")
	}

	var (
		nidAndData  []byte
		requiredLen = w.namespaceSize + len(data)
	)

	if w.byteSlicePool != nil {
		nidAndData = w.byteSlicePool.getOrAlloc(requiredLen)
	} else {
		nidAndData = make([]byte, requiredLen)
	}
	copy(nidAndData[w.namespaceSize:], data)
	// use the parity namespace if the cell is not in Q0 of the extended data square
	if w.isQuadrantZero() {
		copy(nidAndData[:w.namespaceSize], data[:w.namespaceSize])
	} else {
		copy(nidAndData[:w.namespaceSize], w.parityNamespace)
	}
	err := w.tree.Push(nidAndData)

	if err != nil {
		return err
	}
	w.incrementShareIndex()
	return nil
}

// Root fulfills the rsmt2d.Tree interface by generating and returning the
// underlying NamespaceMerkleTree Root.
func (w *erasuredNamespacedMerkleTree) Root() ([]byte, error) {
	root, err := w.tree.Root()
	if err != nil {
		return nil, err
	}
	return root, nil
}

// FastRoot fulfills the rsmt2d.Tree interface by generating and returning the
// underlying NamespaceMerkleTree Root. For this implementation, it behaves the same as Root().
func (w *erasuredNamespacedMerkleTree) FastRoot() ([]byte, error) {
	return w.tree.FastRoot()
}

// incrementShareIndex increments the share index by one.
func (w *erasuredNamespacedMerkleTree) incrementShareIndex() {
	w.shareIndex++
}

// isQuadrantZero returns true if the current share index and axis index are both
// in the original data square.
func (w *erasuredNamespacedMerkleTree) isQuadrantZero() bool {
	return w.shareIndex < w.squareSize && w.axisIndex < w.squareSize
}
