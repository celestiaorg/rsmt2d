package rsmt2d

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"
)

func TestComputeExtendedDataSquare(t *testing.T) {
	codec := codecs[RSGF8].codecType()
	result, err := ComputeExtendedDataSquare([][]byte{
		{1}, {2},
		{3}, {4},
	}, codec, NewDefaultTree)
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(result.square, [][][]byte{
		{{1}, {2}, {7}, {13}},
		{{3}, {4}, {13}, {31}},
		{{5}, {14}, {19}, {41}},
		{{9}, {26}, {47}, {69}},
	}) {
		t.Errorf("NewExtendedDataSquare failed for 2x2 square with chunk size 1")
	}
}

// dump acts as a data dump for the benchmarks to stop the compiler from makeing
// unrealistic optimizations
var dump *ExtendedDataSquare

// BenchmarkExtension benchmarks extending datasquares sizes 4-128 using all supported codecs
func BenchmarkExtension(b *testing.B) {
	for _codecType := range codecs {
		for i := 4; i < 129; i *= 2 {
			square := genRandDS(i)
			b.Run(
				fmt.Sprintf("%s size %d (extended = %d) ", _codecType, i, i*2),
				func(b *testing.B) {
					for n := 0; n < b.N; n++ {
						eds, err := ComputeExtendedDataSquare(square, _codecType)
						if err != nil {
							b.Error(err)
						}
						dump = eds
					}
				},
			)
		}
		fmt.Println("-------------------------------")
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
