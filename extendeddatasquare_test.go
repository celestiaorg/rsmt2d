package rsmt2d

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	zeros     = bytes.Repeat([]byte{0}, shareSize)
	ones      = bytes.Repeat([]byte{1}, shareSize)
	twos      = bytes.Repeat([]byte{2}, shareSize)
	threes    = bytes.Repeat([]byte{3}, shareSize)
	fours     = bytes.Repeat([]byte{4}, shareSize)
	fives     = bytes.Repeat([]byte{5}, shareSize)
	eights    = bytes.Repeat([]byte{8}, shareSize)
	elevens   = bytes.Repeat([]byte{11}, shareSize)
	thirteens = bytes.Repeat([]byte{13}, shareSize)
	fifteens  = bytes.Repeat([]byte{15}, shareSize)
)

//go:embed testdata/edsCustomTree.json
var edsCustomTree []byte

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
			data: [][]byte{ones},
			want: [][][]byte{
				{ones, ones},
				{ones, ones},
			},
		},
		{
			name: "2x2",
			data: [][]byte{
				ones, twos,
				threes, fours,
			},
			want: [][][]byte{
				{ones, twos, zeros, threes},
				{threes, fours, eights, fifteens},
				{twos, elevens, thirteens, fours},
				{zeros, thirteens, fives, eights},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ComputeExtendedDataSquare(tc.data, codec, DefaultTreeName)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, result.squareRow)
		})
	}

	t.Run("returns an error if chunkSize is not a multiple of 64", func(t *testing.T) {
		chunk := bytes.Repeat([]byte{1}, 65)
		_, err := ComputeExtendedDataSquare([][]byte{chunk}, NewLeoRSCodec(), DefaultTreeName)
		assert.Error(t, err)
	})
}

func TestImportExtendedDataSquare(t *testing.T) {
	t.Run("is able to import an EDS", func(t *testing.T) {
		eds := createExampleEds(t, shareSize)
		got, err := ImportExtendedDataSquare(eds.Flattened(), NewLeoRSCodec(), DefaultTreeName)
		assert.NoError(t, err)
		assert.Equal(t, eds.Flattened(), got.Flattened())
	})
	t.Run("returns an error if chunkSize is not a multiple of 64", func(t *testing.T) {
		chunk := bytes.Repeat([]byte{1}, 65)
		_, err := ImportExtendedDataSquare([][]byte{chunk}, NewLeoRSCodec(), DefaultTreeName)
		assert.Error(t, err)
	})
}

func TestMarshalJSON(t *testing.T) {
	original, err := ComputeExtendedDataSquare([][]byte{ones, twos, threes, fours}, NewLeoRSCodec(), DefaultTreeName)
	require.NoError(t, err)

	edsBytes, err := original.MarshalJSON()
	require.NoError(t, err)

	var got ExtendedDataSquare
	err = json.Unmarshal(edsBytes, &got)
	require.NoError(t, err)

	assert.Equal(t, original.dataSquare.Flattened(), got.dataSquare.Flattened())
	assert.Equal(t, original.codec.Name(), got.codec.Name())
	assert.Equal(t, original.treeName, got.treeName)
}

func TestUnmarshalJSON(t *testing.T) {
	t.Run("throws an error when unmarshaling an unregistered custom tree", func(t *testing.T) {
		var eds ExtendedDataSquare
		err := eds.UnmarshalJSON(edsCustomTree)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "custom-tree not registered yet")
	})

	type testCase struct {
		name     string
		original *ExtendedDataSquare
		want     *ExtendedDataSquare
		wantErr  bool
	}

	defaultEDS := exampleEds(t, DefaultTreeName)

	// The tree name is intentionally set to empty to test whether the
	// Unmarshal process appropriately falls back to the default tree
	defaultEDSWithoutTreeName := exampleEds(t, DefaultTreeName)
	defaultEDSWithoutTreeName.treeName = ""

	customTreeName := "custom-tree"
	err := RegisterTree(customTreeName, sudoConstructorFn)
	require.NoError(t, err)
	defer cleanUp(customTreeName)
	customEDS := exampleEds(t, customTreeName)

	testCases := []testCase{
		{
			name:     "can unmarshal the default EDS",
			original: defaultEDS,
			want:     defaultEDS,
			wantErr:  false,
		},
		{
			name:     "can unmarshal the default EDS even if tree name is removed",
			original: defaultEDSWithoutTreeName,
			want:     defaultEDS,
			wantErr:  false,
		},
		{
			name:     "can unmarshal an EDS with a custom tree",
			original: customEDS,
			want:     customEDS,
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			edsBytes, err := json.Marshal(tc.original)
			assert.NoError(t, err)

			var got ExtendedDataSquare
			err = got.UnmarshalJSON(edsBytes)
			assert.NoError(t, err)

			assert.Equal(t, tc.want.dataSquare.Flattened(), got.dataSquare.Flattened())
			assert.Equal(t, tc.want.codec.Name(), got.codec.Name())
			assert.Equal(t, tc.want.treeName, got.treeName)
		})
	}
}

func TestNewExtendedDataSquare(t *testing.T) {
	t.Run("returns an error if edsWidth is not even", func(t *testing.T) {
		edsWidth := uint(1)

		_, err := NewExtendedDataSquare(NewLeoRSCodec(), NewDefaultTree, edsWidth, shareSize)
		assert.Error(t, err)
	})
	t.Run("returns an error if chunkSize is not a multiple of 64", func(t *testing.T) {
		edsWidth := uint(1)
		chunkSize := uint(65)

		_, err := NewExtendedDataSquare(NewLeoRSCodec(), NewDefaultTree, edsWidth, chunkSize)
		assert.Error(t, err)
	})
	t.Run("returns a 4x4 EDS", func(t *testing.T) {
		edsWidth := uint(4)

		got, err := NewExtendedDataSquare(NewLeoRSCodec(), NewDefaultTree, edsWidth, shareSize)
		assert.NoError(t, err)
		assert.Equal(t, edsWidth, got.width)
		assert.Equal(t, uint(shareSize), got.chunkSize)
	})
	t.Run("returns a 4x4 EDS that can be populated via SetCell", func(t *testing.T) {
		edsWidth := uint(4)

		got, err := NewExtendedDataSquare(NewLeoRSCodec(), NewDefaultTree, edsWidth, shareSize)
		assert.NoError(t, err)

		chunk := bytes.Repeat([]byte{1}, int(shareSize))
		err = got.SetCell(0, 0, chunk)
		assert.NoError(t, err)
		assert.Equal(t, chunk, got.squareRow[0][0])
	})
	t.Run("returns an error when SetCell is invoked on an EDS with a chunk that is not the correct size", func(t *testing.T) {
		edsWidth := uint(4)
		incorrectChunkSize := shareSize + 1

		got, err := NewExtendedDataSquare(NewLeoRSCodec(), NewDefaultTree, edsWidth, shareSize)
		assert.NoError(t, err)

		chunk := bytes.Repeat([]byte{1}, incorrectChunkSize)
		err = got.SetCell(0, 0, chunk)
		assert.Error(t, err)
	})
}

func TestImmutableRoots(t *testing.T) {
	codec := NewLeoRSCodec()
	result, err := ComputeExtendedDataSquare([][]byte{
		ones, twos,
		threes, fours,
	}, codec, DefaultTreeName)
	if err != nil {
		panic(err)
	}

	mutatedRowRoots, err := result.RowRoots()
	assert.NoError(t, err)

	mutatedRowRoots[0][0]++ // mutate

	rowRoots, err := result.RowRoots()
	assert.NoError(t, err)

	if reflect.DeepEqual(mutatedRowRoots, rowRoots) {
		t.Errorf("Exported EDS RowRoots was mutable")
	}

	mutatedColRoots, err := result.ColRoots()
	assert.NoError(t, err)

	mutatedColRoots[0][0]++ // mutate

	colRoots, err := result.ColRoots()
	assert.NoError(t, err)

	if reflect.DeepEqual(mutatedColRoots, colRoots) {
		t.Errorf("Exported EDS ColRoots was mutable")
	}
}

func TestEDSRowColImmutable(t *testing.T) {
	codec := NewLeoRSCodec()
	result, err := ComputeExtendedDataSquare([][]byte{
		ones, twos,
		threes, fours,
	}, codec, DefaultTreeName)
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

func TestRowRoots(t *testing.T) {
	t.Run("returns row roots for a 4x4 EDS", func(t *testing.T) {
		eds, err := ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, NewLeoRSCodec(), DefaultTreeName)
		require.NoError(t, err)

		rowRoots, err := eds.RowRoots()
		assert.NoError(t, err)
		assert.Len(t, rowRoots, 4)
	})

	t.Run("returns an error for an incomplete EDS", func(t *testing.T) {
		eds, err := ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, NewLeoRSCodec(), DefaultTreeName)
		require.NoError(t, err)

		// set a cell to nil to make the EDS incomplete
		eds.setCell(0, 0, nil)

		_, err = eds.RowRoots()
		assert.Error(t, err)
	})
}

func TestColRoots(t *testing.T) {
	t.Run("returns col roots for a 4x4 EDS", func(t *testing.T) {
		eds, err := ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, NewLeoRSCodec(), DefaultTreeName)
		require.NoError(t, err)

		colRoots, err := eds.ColRoots()
		assert.NoError(t, err)
		assert.Len(t, colRoots, 4)
	})

	t.Run("returns an error for an incomplete EDS", func(t *testing.T) {
		eds, err := ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, NewLeoRSCodec(), DefaultTreeName)
		require.NoError(t, err)

		// set a cell to nil to make the EDS incomplete
		eds.setCell(0, 0, nil)

		_, err = eds.ColRoots()
		assert.Error(t, err)
	})
}

// dump acts as a data dump for the benchmarks to stop the compiler from making
// unrealistic optimizations
var dump *ExtendedDataSquare

// BenchmarkExtension benchmarks extending datasquares sizes 4-128 using all
// supported codecs (encoding only)
func BenchmarkExtensionEncoding(b *testing.B) {
	for i := 4; i < 513; i *= 2 {
		for codecName, codec := range codecs {
			if codec.MaxChunks() < i*i {
				// Only test codecs that support this many chunks
				continue
			}

			square := genRandDS(i, shareSize)
			b.Run(
				fmt.Sprintf("%s %dx%dx%d ODS", codecName, i, i, len(square[0])),
				func(b *testing.B) {
					for n := 0; n < b.N; n++ {
						eds, err := ComputeExtendedDataSquare(square, codec, DefaultTreeName)
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
			if codec.MaxChunks() < i*i {
				// Only test codecs that support this many chunks
				continue
			}

			square := genRandDS(i, shareSize)
			b.Run(
				fmt.Sprintf("%s %dx%dx%d ODS", codecName, i, i, len(square[0])),
				func(b *testing.B) {
					for n := 0; n < b.N; n++ {
						eds, err := ComputeExtendedDataSquare(square, codec, DefaultTreeName)
						if err != nil {
							b.Error(err)
						}
						_, _ = eds.RowRoots()
						_, _ = eds.ColRoots()
						dump = eds
					}
				},
			)
		}
	}
}

// genRandDS make a datasquare of random data, with width describing the number
// of shares on a single side of the ds
func genRandDS(width int, chunkSize int) [][]byte {
	var ds [][]byte
	count := width * width
	for i := 0; i < count; i++ {
		share := make([]byte, chunkSize)
		_, err := rand.Read(share)
		if err != nil {
			panic(err)
		}
		ds = append(ds, share)
	}
	return ds
}

// TestFlattened_EDS tests that eds.Flattened() returns all the shares in the
// EDS. This function has the `_EDS` suffix to avoid a name collision with the
// TestFlattened.
func TestFlattened_EDS(t *testing.T) {
	example := createExampleEds(t, shareSize)
	want := [][]byte{
		ones, twos, zeros, threes,
		threes, fours, eights, fifteens,
		twos, elevens, thirteens, fours,
		zeros, thirteens, fives, eights,
	}

	got := example.Flattened()
	assert.Equal(t, want, got)
}

func TestFlattenedODS(t *testing.T) {
	example := createExampleEds(t, shareSize)
	want := [][]byte{
		ones, twos,
		threes, fours,
	}

	got := example.FlattenedODS()
	assert.Equal(t, want, got)
}

func TestEquals(t *testing.T) {
	t.Run("returns true for two equal EDS", func(t *testing.T) {
		a := createExampleEds(t, shareSize)
		b := createExampleEds(t, shareSize)
		assert.True(t, a.Equals(b))
	})
	t.Run("returns false for two unequal EDS", func(t *testing.T) {
		a := createExampleEds(t, shareSize)

		type testCase struct {
			name  string
			other *ExtendedDataSquare
		}

		unequalOriginalDataWidth := createExampleEds(t, shareSize)
		unequalOriginalDataWidth.originalDataWidth = 1

		unequalCodecs := createExampleEds(t, shareSize)
		unequalCodecs.codec = newTestCodec()

		unequalChunkSize := createExampleEds(t, shareSize*2)

		unequalEds, err := ComputeExtendedDataSquare([][]byte{ones}, NewLeoRSCodec(), DefaultTreeName)
		require.NoError(t, err)

		testCases := []testCase{
			{
				name:  "unequal original data width",
				other: unequalOriginalDataWidth,
			},
			{
				name:  "unequal codecs",
				other: unequalCodecs,
			},
			{
				name:  "unequal chunkSize",
				other: unequalChunkSize,
			},
			{
				name:  "unequalEds",
				other: unequalEds,
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.False(t, a.Equals(tc.other))
				assert.False(t, reflect.DeepEqual(a, tc.other))
			})
		}
	})
}

func TestRoots(t *testing.T) {
	t.Run("returns roots for a 4x4 EDS", func(t *testing.T) {
		eds, err := ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, NewLeoRSCodec(), DefaultTreeName)
		require.NoError(t, err)

		roots, err := eds.Roots()
		require.NoError(t, err)
		assert.Len(t, roots, 8)

		rowRoots, err := eds.RowRoots()
		require.NoError(t, err)

		colRoots, err := eds.ColRoots()
		require.NoError(t, err)

		assert.Equal(t, roots[0], rowRoots[0])
		assert.Equal(t, roots[1], rowRoots[1])
		assert.Equal(t, roots[2], rowRoots[2])
		assert.Equal(t, roots[3], rowRoots[3])
		assert.Equal(t, roots[4], colRoots[0])
		assert.Equal(t, roots[5], colRoots[1])
		assert.Equal(t, roots[6], colRoots[2])
		assert.Equal(t, roots[7], colRoots[3])
	})

	t.Run("returns an error for an incomplete EDS", func(t *testing.T) {
		eds, err := ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, NewLeoRSCodec(), DefaultTreeName)
		require.NoError(t, err)

		// set a cell to nil to make the EDS incomplete
		eds.setCell(0, 0, nil)

		_, err = eds.Roots()
		assert.Error(t, err)
	})
}

func createExampleEds(t *testing.T, chunkSize int) (eds *ExtendedDataSquare) {
	ones := bytes.Repeat([]byte{1}, chunkSize)
	twos := bytes.Repeat([]byte{2}, chunkSize)
	threes := bytes.Repeat([]byte{3}, chunkSize)
	fours := bytes.Repeat([]byte{4}, chunkSize)
	ods := [][]byte{
		ones, twos,
		threes, fours,
	}

	eds, err := ComputeExtendedDataSquare(ods, NewLeoRSCodec(), DefaultTreeName)
	require.NoError(t, err)
	return eds
}

func exampleEds(t *testing.T, treeName string) *ExtendedDataSquare {
	eds, err := ComputeExtendedDataSquare([][]byte{ones, twos, threes, fours}, NewLeoRSCodec(), treeName)
	require.NoError(t, err)
	return eds
}
