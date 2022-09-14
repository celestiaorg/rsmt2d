package rsmt2d

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"
)

func TestComputeExtendedDataSquare(t *testing.T) {
	codec := NewRSGF8Codec()
	result, err := ComputeExtendedDataSquare([][]byte{
		{1}, {2},
		{3}, {4},
	}, codec, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(result.squareRow, [][][]byte{
		{{1}, {2}, {7}, {13}},
		{{3}, {4}, {13}, {31}},
		{{5}, {14}, {19}, {41}},
		{{9}, {26}, {47}, {69}},
	}) {
		t.Errorf("NewExtendedDataSquare failed for 2x2 square with chunk size 1")
	}
}

func TestImmutableRoots(t *testing.T) {
	codec := NewRSGF8Codec()
	result, err := ComputeExtendedDataSquare([][]byte{
		{1}, {2},
		{3}, {4},
	}, codec, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	row := result.RowRoots()
	row[0][0]++
	if reflect.DeepEqual(row, result.RowRoots()) {
		t.Errorf("Exported EDS RowRoots was mutable")
	}

	col := result.ColRoots()
	col[0][0]++
	if reflect.DeepEqual(col, result.ColRoots()) {
		t.Errorf("Exported EDS ColRoots was mutable")
	}
}

func TestEDSRowColImmutable(t *testing.T) {
	codec := NewRSGF8Codec()
	result, err := ComputeExtendedDataSquare([][]byte{
		{1}, {2},
		{3}, {4},
	}, codec, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	row := result.Row(0)
	row[0][0]++
	if reflect.DeepEqual(row, result.Row(0)) {
		t.Errorf("Exported EDS Row was mutable")
	}

	col := result.Col(0)
	col[0][0]++
	if reflect.DeepEqual(col, result.Col(0)) {
		t.Errorf("Exported EDS Col was mutable")
	}
}

// dump acts as a data dump for the benchmarks to stop the compiler from making
// unrealistic optimizations
var dump *ExtendedDataSquare

// BenchmarkExtension benchmarks extending datasquares sizes 4-128 using all supported codecs
func BenchmarkExtension(b *testing.B) {
	for i := 4; i < 129; i *= 2 {
		for codecName, codec := range codecs {
			square := genRandDS(i)
			b.Run(
				fmt.Sprintf("%s size %d", codecName, i),
				func(b *testing.B) {
					for n := 0; n < b.N; n++ {
						eds, err := ComputeExtendedDataSquare(square, codec, NewDefaultTree)
						if err != nil {
							b.Error(err)
						}
						_ = eds.RowRoots()
						_ = eds.ColRoots()
						dump = eds
					}
				},
			)
		}

	}
}

// genRandDS make a datasquare of random data, with width describing the number
// of shares on a single side of the ds
func genRandDS(width int) [][]byte {
	var ds [][]byte
	count := width * width
	for i := 0; i < count; i++ {
		share := make([]byte, 256)
		rand.Read(share)
		ds = append(ds, share)
	}
	return ds
}
