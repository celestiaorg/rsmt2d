package rsmt2d

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	zero     = bytes.Repeat([]byte{0}, 64)
	one      = bytes.Repeat([]byte{1}, 64)
	two      = bytes.Repeat([]byte{2}, 64)
	three    = bytes.Repeat([]byte{3}, 64)
	four     = bytes.Repeat([]byte{4}, 64)
	five     = bytes.Repeat([]byte{5}, 64)
	eight    = bytes.Repeat([]byte{8}, 64)
	eleven   = bytes.Repeat([]byte{11}, 64)
	thirteen = bytes.Repeat([]byte{13}, 64)
	fifteen  = bytes.Repeat([]byte{15}, 64)
)

func TestComputeExtendedDataSquare(t *testing.T) {
	codec := NewLeoRSCodec()

	type testCase struct {
		name string
		data [][]byte
		want [][][]byte
	}
	testCases := []testCase{
		{
			name: "1x1",
			data: [][]byte{one},
			want: [][][]byte{
				{one, one},
				{one, one},
			},
		},
		{
			name: "2x2",
			data: [][]byte{
				one, two,
				three, four,
			},
			want: [][][]byte{
				{one, two, zero, three},
				{three, four, eight, fifteen},
				{two, eleven, thirteen, four},
				{zero, thirteen, five, eight},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ComputeExtendedDataSquare(tc.data, codec, NewDefaultTree)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, result.squareRow)
		})
	}
}

func TestMarshalJSON(t *testing.T) {
	codec := NewLeoRSCodec()
	result, err := ComputeExtendedDataSquare([][]byte{
		one, two,
		three, four,
	}, codec, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	edsBytes, err := json.Marshal(result)
	if err != nil {
		t.Errorf("failed to marshal EDS: %v", err)
	}

	var eds ExtendedDataSquare
	err = json.Unmarshal(edsBytes, &eds)
	if err != nil {
		t.Errorf("failed to marshal EDS: %v", err)
	}
	if !reflect.DeepEqual(result.squareRow, eds.squareRow) {
		t.Errorf("eds not equal after json marshal/unmarshal")
	}
}

func TestImmutableRoots(t *testing.T) {
	codec := NewLeoRSCodec()
	result, err := ComputeExtendedDataSquare([][]byte{
		one, two,
		three, four,
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
	codec := NewLeoRSCodec()
	result, err := ComputeExtendedDataSquare([][]byte{
		one, two,
		three, four,
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

// BenchmarkExtension benchmarks extending datasquares sizes 4-128 using all
// supported codecs (encoding only)
func BenchmarkExtensionEncoding(b *testing.B) {
	for i := 4; i < 513; i *= 2 {
		for codecName, codec := range codecs {
			if codec.maxChunks() < i*i {
				// Only test codecs that support this many chunks
				continue
			}

			square := genRandDS(i)
			b.Run(
				fmt.Sprintf("%s %dx%dx%d ODS", codecName, i, i, len(square[0])),
				func(b *testing.B) {
					for n := 0; n < b.N; n++ {
						eds, err := ComputeExtendedDataSquare(square, codec, NewDefaultTree)
						if err != nil {
							b.Error(err)
						}
						dump = eds
					}
				},
			)
		}

	}
}

// BenchmarkExtension benchmarks extending datasquares sizes 4-128 using all
// supported codecs (both encoding and root computation)
func BenchmarkExtensionWithRoots(b *testing.B) {
	for i := 4; i < 513; i *= 2 {
		for codecName, codec := range codecs {
			if codec.maxChunks() < i*i {
				// Only test codecs that support this many chunks
				continue
			}

			square := genRandDS(i)
			b.Run(
				fmt.Sprintf("%s %dx%dx%d ODS", codecName, i, i, len(square[0])),
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
