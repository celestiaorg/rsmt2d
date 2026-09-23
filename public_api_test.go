package rsmt2d_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/celestiaorg/rsmt2d"
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

func TestComputeExtendedDataSquare(t *testing.T) {
	codec := rsmt2d.NewLeoRSCodec()

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
			result, err := rsmt2d.ComputeExtendedDataSquare(tc.data, codec, rsmt2d.NewDefaultTree)
			assert.NoError(t, err)
			for i, row := range tc.want {
				assert.Equal(t, row, result.Row(uint(i)))
			}
		})
	}

	t.Run("returns an error if shareSize is not a multiple of 64", func(t *testing.T) {
		share := bytes.Repeat([]byte{1}, 65)
		_, err := rsmt2d.ComputeExtendedDataSquare([][]byte{share}, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		assert.Error(t, err)
	})
}

func TestImportExtendedDataSquare(t *testing.T) {
	t.Run("is able to import an EDS", func(t *testing.T) {
		eds := createExampleEds(t, shareSize)
		got, err := rsmt2d.ImportExtendedDataSquare(eds.Flattened(), rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		assert.NoError(t, err)
		assert.Equal(t, eds.Flattened(), got.Flattened())
	})
	t.Run("returns an error if shareSize is not a multiple of 64", func(t *testing.T) {
		share := bytes.Repeat([]byte{1}, 65)
		_, err := rsmt2d.ImportExtendedDataSquare([][]byte{share}, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		assert.Error(t, err)
	})
}

func TestMarshalJSON(t *testing.T) {
	codec := rsmt2d.NewLeoRSCodec()
	result, err := rsmt2d.ComputeExtendedDataSquare([][]byte{
		ones, twos,
		threes, fours,
	}, codec, rsmt2d.NewDefaultTree)
	if err != nil {
		panic(err)
	}

	edsBytes, err := json.Marshal(result)
	if err != nil {
		t.Errorf("failed to marshal EDS: %v", err)
	}

	var eds rsmt2d.ExtendedDataSquare
	err = json.Unmarshal(edsBytes, &eds)
	if err != nil {
		t.Errorf("failed to marshal EDS: %v", err)
	}
	if !reflect.DeepEqual(result.Flattened(), eds.Flattened()) {
		t.Errorf("eds not equal after json marshal/unmarshal")
	}
}

func TestNewExtendedDataSquare(t *testing.T) {
	t.Run("returns an error if edsWidth is not even", func(t *testing.T) {
		edsWidth := uint(1)

		_, err := rsmt2d.NewExtendedDataSquare(rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree, edsWidth, shareSize)
		assert.Error(t, err)
	})
	t.Run("returns an error if shareSize is not a multiple of 64", func(t *testing.T) {
		edsWidth := uint(1)
		shareSize := uint(65)

		_, err := rsmt2d.NewExtendedDataSquare(rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree, edsWidth, shareSize)
		assert.Error(t, err)
	})
	t.Run("returns a 4x4 EDS", func(t *testing.T) {
		edsWidth := uint(4)

		got, err := rsmt2d.NewExtendedDataSquare(rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree, edsWidth, shareSize)
		assert.NoError(t, err)
		assert.Equal(t, edsWidth, got.Width())
	})
	t.Run("returns a 4x4 EDS that can be populated via SetCell", func(t *testing.T) {
		edsWidth := uint(4)

		got, err := rsmt2d.NewExtendedDataSquare(rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree, edsWidth, shareSize)
		assert.NoError(t, err)

		share := bytes.Repeat([]byte{1}, int(shareSize))
		err = got.SetCell(0, 0, share)
		assert.NoError(t, err)
		assert.Equal(t, share, got.GetCell(0, 0))
	})
	t.Run("returns an error when SetCell is invoked on an EDS with a share that is not the correct size", func(t *testing.T) {
		edsWidth := uint(4)
		incorrectShareSize := shareSize + 1

		got, err := rsmt2d.NewExtendedDataSquare(rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree, edsWidth, shareSize)
		assert.NoError(t, err)

		share := bytes.Repeat([]byte{1}, incorrectShareSize)
		err = got.SetCell(0, 0, share)
		assert.Error(t, err)
	})
}

func TestImmutableRoots(t *testing.T) {
	codec := rsmt2d.NewLeoRSCodec()
	result, err := rsmt2d.ComputeExtendedDataSquare([][]byte{
		ones, twos,
		threes, fours,
	}, codec, rsmt2d.NewDefaultTree)
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
	codec := rsmt2d.NewLeoRSCodec()
	result, err := rsmt2d.ComputeExtendedDataSquare([][]byte{
		ones, twos,
		threes, fours,
	}, codec, rsmt2d.NewDefaultTree)
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
		eds, err := rsmt2d.ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		require.NoError(t, err)

		rowRoots, err := eds.RowRoots()
		assert.NoError(t, err)
		assert.Len(t, rowRoots, 4)
	})

	t.Run("returns an error for an incomplete EDS", func(t *testing.T) {
		eds, err := rsmt2d.ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		require.NoError(t, err)

		// set a cell to nil to make the EDS incomplete
		shares := eds.Flattened()
		shares[0] = nil
		eds, err = rsmt2d.ImportExtendedDataSquare(shares, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		require.NoError(t, err)

		_, err = eds.RowRoots()
		assert.Error(t, err)
	})
}

func TestColRoots(t *testing.T) {
	t.Run("returns col roots for a 4x4 EDS", func(t *testing.T) {
		eds, err := rsmt2d.ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		require.NoError(t, err)

		colRoots, err := eds.ColRoots()
		assert.NoError(t, err)
		assert.Len(t, colRoots, 4)
	})

	t.Run("returns an error for an incomplete EDS", func(t *testing.T) {
		eds, err := rsmt2d.ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		require.NoError(t, err)

		// set a cell to nil to make the EDS incomplete
		shares := eds.Flattened()
		shares[0] = nil
		eds, err = rsmt2d.ImportExtendedDataSquare(shares, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		require.NoError(t, err)

		_, err = eds.ColRoots()
		assert.Error(t, err)
	})
}

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

func TestRoots(t *testing.T) {
	t.Run("returns roots for a 4x4 EDS", func(t *testing.T) {
		eds, err := rsmt2d.ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
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
		eds, err := rsmt2d.ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		require.NoError(t, err)

		// set a cell to nil to make the EDS incomplete
		shares := eds.Flattened()
		shares[0] = nil
		eds, err = rsmt2d.ImportExtendedDataSquare(shares, rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree)
		require.NoError(t, err)

		_, err = eds.Roots()
		assert.Error(t, err)
	})
}

func TestAxisString(t *testing.T) {
	assert.Equal(t, "row", rsmt2d.Row.String())
	assert.Equal(t, "col", rsmt2d.Col.String())
	assert.Panics(t, func() { _ = rsmt2d.Axis(2).String() })
}

func TestErrByzantineDataError(t *testing.T) {
	assert.Equal(t, "byzantine row: 0", (&rsmt2d.ErrByzantineData{rsmt2d.Row, 0, nil}).Error())
	assert.Equal(t, "byzantine col: 3", (&rsmt2d.ErrByzantineData{rsmt2d.Col, 3, nil}).Error())
}

func TestUnmarshalJSONErrors(t *testing.T) {
	t.Run("malformed JSON", func(t *testing.T) {
		var eds rsmt2d.ExtendedDataSquare
		require.Error(t, eds.UnmarshalJSON([]byte(`{"data_square": [`)))
	})
	t.Run("unregistered codec", func(t *testing.T) {
		var eds rsmt2d.ExtendedDataSquare
		err := eds.UnmarshalJSON([]byte(`{"data_square": ["AQ==", "AQ==", "AQ==", "AQ=="], "codec": "no-such-codec"}`))
		require.ErrorContains(t, err, "no-such-codec")
	})
	t.Run("data square is not square", func(t *testing.T) {
		share := base64.StdEncoding.EncodeToString(make([]byte, 64))
		var eds rsmt2d.ExtendedDataSquare
		err := eds.UnmarshalJSON(fmt.Appendf(nil, `{"data_square": [%q, %q, %q], "codec": "Leopard"}`, share, share, share))
		require.ErrorContains(t, err, "square number")
	})
}

func TestLeoRSCodecErrors(t *testing.T) {
	codec := rsmt2d.NewLeoRSCodec()

	t.Run("encode with no shares", func(t *testing.T) {
		_, err := codec.Encode(nil)
		require.Error(t, err)
	})
	t.Run("decode with no shares", func(t *testing.T) {
		_, err := codec.Decode(nil)
		require.Error(t, err)
	})
	t.Run("encode with uneven shares", func(t *testing.T) {
		_, err := codec.Encode([][]byte{bytes.Repeat([]byte{1}, 64), bytes.Repeat([]byte{2}, 128)})
		require.Error(t, err)
	})
}
