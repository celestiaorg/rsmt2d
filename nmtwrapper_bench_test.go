package rsmt2d

import (
	"testing"
	"github.com/celestiaorg/nmt"
)

// BenchmarkTreePooling benchmarks the performance of tree creation with pooling
func BenchmarkTreePooling(b *testing.B) {
	const squareSize = 64
	const shareSize = 256
	
	// Create constructor with pooling
	constructor := newErasuredNamespacedMerkleTreeConstructor(
		squareSize,
		nmt.NamespaceIDSize(8),
		nmt.IgnoreMaxNamespace(true),
	)
	
	// Create some sample data
	data := make([]byte, shareSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Get tree from pool
			tree := constructor(Row, 0)
			
			// Use the tree
			for i := 0; i < 10; i++ {
				_ = tree.Push(data)
			}
			_, _ = tree.ConsumeRoot()
			
			// Release tree back to pool
			ReleaseTree(tree)
		}
	})
}

// BenchmarkTreeNoPooling benchmarks the performance without pooling for comparison
func BenchmarkTreeNoPooling(b *testing.B) {
	const squareSize = 64
	const shareSize = 256
	
	// Create some sample data
	data := make([]byte, shareSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	
	opts := []nmt.Option{
		nmt.NamespaceIDSize(8),
		nmt.IgnoreMaxNamespace(true),
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Create new tree without pooling
			tree := newErasuredNamespacedMerkleTree(squareSize, 0, opts...)
			
			// Use the tree
			for i := 0; i < 10; i++ {
				_ = tree.Push(data)
			}
			_, _ = tree.ConsumeRoot()
		}
	})
}