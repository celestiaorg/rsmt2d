package rsmt2d

import (
	"testing"
)

// BenchmarkAccessMethods benchmarks the performance of accessor methods
// to demonstrate the performance improvement from removing deepcopy
func BenchmarkAccessMethods(b *testing.B) {
	codec := NewLeoRSCodec()
	
	// Create a reasonably sized EDS (16x16 original data -> 32x32 extended)
	square := genRandDS(16, shareSize)
	eds, err := ComputeExtendedDataSquare(square, codec, NewDefaultTree)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("Row access", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = eds.Row(0)
		}
	})

	b.Run("Col access", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = eds.Col(0)
		}
	})

	b.Run("RowRoots access", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = eds.RowRoots()
		}
	})

	b.Run("ColRoots access", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = eds.ColRoots()
		}
	})

	b.Run("Flattened access", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = eds.Flattened()
		}
	})
}

// BenchmarkWithDeepCopy provides a comparison showing what performance 
// would be like if we still used deepCopy (for reference)
func BenchmarkWithDeepCopy(b *testing.B) {
	codec := NewLeoRSCodec()
	
	// Create a reasonably sized EDS (16x16 original data -> 32x32 extended)
	square := genRandDS(16, shareSize)
	eds, err := ComputeExtendedDataSquare(square, codec, NewDefaultTree)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("Row access with deepCopy", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = deepCopy(eds.Row(0))
		}
	})

	b.Run("RowRoots access with deepCopy", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			roots, _ := eds.RowRoots()
			_ = deepCopy(roots)
		}
	})
}