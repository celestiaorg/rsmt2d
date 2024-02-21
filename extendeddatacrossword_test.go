package rsmt2d

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/celestiaorg/nmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chunkSize is the size of each chunk in bytes. This value is used for testing.
const chunkSize = 512

// PseudoFraudProof is an example fraud proof.
// TODO a real fraud proof would have a Merkle proof for each chunk.
type PseudoFraudProof struct {
	Mode   int      // Row (0) or column (1)
	Index  uint     // Row or column index
	Chunks [][]byte // Bad chunks (nil are missing)
}

func TestRepairExtendedDataSquare(t *testing.T) {
	codec := NewLeoRSCodec()
	original := createTestEds(codec, chunkSize)

	rowRoots, err := original.RowRoots()
	require.NoError(t, err)

	colRoots, err := original.ColRoots()
	require.NoError(t, err)

	// Verify that an EDS can be repaired after the maximum amount of erasures
	t.Run("MaximumErasures", func(t *testing.T) {
		flattened := original.Flattened()
		flattened[0], flattened[2], flattened[3] = nil, nil, nil
		flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
		flattened[8], flattened[9], flattened[10] = nil, nil, nil
		flattened[12], flattened[13] = nil, nil

		// Re-import the data square.
		eds, err := ImportExtendedDataSquare(flattened, codec, NewDefaultTree)
		if err != nil {
			t.Errorf("ImportExtendedDataSquare failed: %v", err)
		}

		err = eds.Repair(rowRoots, colRoots)
		if err != nil {
			t.Errorf("unexpected err while repairing data square: %v, codec: :%s", err, codec.Name())
		} else {
			assert.Equal(t, original.GetCell(0, 0), bytes.Repeat([]byte{1}, chunkSize))
			assert.Equal(t, original.GetCell(0, 1), bytes.Repeat([]byte{2}, chunkSize))
			assert.Equal(t, original.GetCell(1, 0), bytes.Repeat([]byte{3}, chunkSize))
			assert.Equal(t, original.GetCell(1, 1), bytes.Repeat([]byte{4}, chunkSize))
		}
	})

	// Verify that an EDS returns an error when there are too many erasures
	t.Run("Unrepairable", func(t *testing.T) {
		flattened := original.Flattened()
		flattened[0], flattened[2], flattened[3] = nil, nil, nil
		flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
		flattened[8], flattened[9], flattened[10] = nil, nil, nil
		flattened[12], flattened[13], flattened[14] = nil, nil, nil

		// Re-import the data square.
		eds, err := ImportExtendedDataSquare(flattened, codec, NewDefaultTree)
		if err != nil {
			t.Errorf("ImportExtendedDataSquare failed: %v", err)
		}

		err = eds.Repair(rowRoots, colRoots)
		if err != ErrUnrepairableDataSquare {
			t.Errorf("did not return an error on trying to repair an unrepairable square")
		}
	})
}

func TestValidFraudProof(t *testing.T) {
	codec := NewLeoRSCodec()

	corruptChunk := bytes.Repeat([]byte{66}, chunkSize)

	original := createTestEds(codec, chunkSize)

	var byzData *ErrByzantineData
	corrupted, err := original.deepCopy(codec)
	if err != nil {
		t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codec.Name())
	}
	corrupted.setCell(0, 0, corruptChunk)
	assert.NoError(t, err)

	rowRoots, err := corrupted.getRowRoots()
	assert.NoError(t, err)

	colRoots, err := corrupted.getColRoots()
	assert.NoError(t, err)

	err = corrupted.Repair(rowRoots, colRoots)
	errors.As(err, &byzData)

	// Construct the fraud proof
	fraudProof := PseudoFraudProof{0, byzData.Index, byzData.Shares}
	// Verify the fraud proof
	// TODO in a real fraud proof, also verify Merkle proof for each non-nil chunk.
	rebuiltChunks, err := codec.Decode(fraudProof.Chunks)
	if err != nil {
		t.Errorf("could not decode fraud proof chunks; got: %v", err)
	}
	root, err := corrupted.computeChunksRoot(rebuiltChunks, byzData.Axis, fraudProof.Index)
	assert.NoError(t, err)
	rowRoot, err := corrupted.getRowRoot(fraudProof.Index)
	assert.NoError(t, err)
	if bytes.Equal(root, rowRoot) {
		// If the roots match, then the fraud proof should be for invalid erasure coding.
		parityChunks, err := codec.Encode(rebuiltChunks[0:corrupted.originalDataWidth])
		if err != nil {
			t.Errorf("could not encode fraud proof chunks; %v", fraudProof)
		}
		startIndex := len(rebuiltChunks) - int(corrupted.originalDataWidth)
		if bytes.Equal(flattenChunks(parityChunks), flattenChunks(rebuiltChunks[startIndex:])) {
			t.Errorf("invalid fraud proof %v", fraudProof)
		}
	}
}

func TestCannotRepairSquareWithBadRoots(t *testing.T) {
	codec := NewLeoRSCodec()

	corruptChunk := bytes.Repeat([]byte{66}, chunkSize)
	original := createTestEds(codec, chunkSize)

	rowRoots, err := original.RowRoots()
	require.NoError(t, err)

	colRoots, err := original.ColRoots()
	require.NoError(t, err)

	original.setCell(0, 0, corruptChunk)
	require.NoError(t, err)
	err = original.Repair(rowRoots, colRoots)
	if err == nil {
		t.Errorf("did not return an error on trying to repair a square with bad roots")
	}
}

func TestCorruptedEdsReturnsErrByzantineData(t *testing.T) {
	corruptChunk := bytes.Repeat([]byte{66}, chunkSize)

	tests := []struct {
		name   string
		coords [][]uint
		values [][]byte
	}{
		{
			name:   "corrupt a chunk in the original data square",
			coords: [][]uint{{0, 0}},
			values: [][]byte{corruptChunk},
		},
		{
			name:   "corrupt a chunk in the extended data square",
			coords: [][]uint{{0, 3}},
			values: [][]byte{corruptChunk},
		},
		{
			name:   "corrupt a chunk at (0, 0) and delete chunks from the rest of the row",
			coords: [][]uint{{0, 0}, {0, 1}, {0, 2}, {0, 3}},
			values: [][]byte{corruptChunk, nil, nil, nil},
		},
		{
			name:   "corrupt a chunk at (3, 0) and delete part of the first row ",
			coords: [][]uint{{3, 0}, {0, 1}, {0, 2}, {0, 3}},
			values: [][]byte{corruptChunk, nil, nil, nil},
		},
		{
			// This test case sets all chunks along the diagonal to nil so that
			// the prerepairSanityCheck does not return an error and it can
			// verify that solveCrossword returns an ErrByzantineData with
			// shares populated.
			name: "set all chunks along the diagonal to nil and then corrupt the cell at (0, 1)",
			// In the ASCII diagram below, _ represents a nil chunk and C
			// represents a corrupted chunk.
			//
			// _ C O O
			// O _ O O
			// O O _ O
			// O O O _
			coords: [][]uint{{0, 0}, {1, 1}, {2, 2}, {3, 3}, {0, 1}},
			values: [][]byte{nil, nil, nil, nil, corruptChunk},
		},
	}

	codec := NewLeoRSCodec()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			eds := createTestEds(codec, chunkSize)

			// compute the rowRoots prior to corruption
			rowRoots, err := eds.getRowRoots()
			assert.NoError(t, err)

			// compute the colRoots prior to corruption
			colRoots, err := eds.getColRoots()
			assert.NoError(t, err)

			for i, coords := range test.coords {
				x := coords[0]
				y := coords[1]
				eds.setCell(x, y, test.values[i])
			}

			err = eds.Repair(rowRoots, colRoots)
			assert.Error(t, err)

			// due to parallelisation, the ErrByzantineData axis may be either row or col
			var byzData *ErrByzantineData
			assert.ErrorAs(t, err, &byzData, "did not return a ErrByzantineData for a bad col or row")
			assert.NotEmpty(t, byzData.Shares)
			assert.Contains(t, byzData.Shares, corruptChunk)
		})
	}
}

func BenchmarkRepair(b *testing.B) {
	// For different ODS sizes
	for originalDataWidth := 4; originalDataWidth <= 512; originalDataWidth *= 2 {
		codec := NewLeoRSCodec()
		if codec.MaxChunks() < originalDataWidth*originalDataWidth {
			// Only test codecs that support this many chunks
			continue
		}

		// Generate a new range original data square then extend it
		square := genRandDataSquare(originalDataWidth, chunkSize)
		eds, err := ComputeExtendedDataSquare(square, codec, NewDefaultTree)
		if err != nil {
			b.Error(err)
		}

		extendedDataWidth := originalDataWidth * 2
		rowRoots, err := eds.RowRoots()
		assert.NoError(b, err)

		colRoots, err := eds.ColRoots()
		assert.NoError(b, err)

		b.Run(
			fmt.Sprintf(
				"%s %dx%dx%d ODS",
				codec.Name(),
				originalDataWidth,
				originalDataWidth,
				len(square[0]),
			),
			func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					b.StopTimer()

					flattened := eds.Flattened()
					// Randomly remove 1/2 of the chunks of each row
					for r := 0; r < extendedDataWidth; r++ {
						for c := 0; c < originalDataWidth; {
							ind := rand.Intn(extendedDataWidth)
							if flattened[r*extendedDataWidth+ind] == nil {
								continue
							}
							flattened[r*extendedDataWidth+ind] = nil
							c++
						}
					}

					// Re-import the data square.
					eds, _ = ImportExtendedDataSquare(flattened, codec, NewDefaultTree)

					b.StartTimer()

					err := eds.Repair(
						rowRoots,
						colRoots,
					)
					if err != nil {
						b.Error(err)
					}
				}
			},
		)
	}
}

func createTestEds(codec Codec, chunkSize int) *ExtendedDataSquare {
	ones := bytes.Repeat([]byte{1}, chunkSize)
	twos := bytes.Repeat([]byte{2}, chunkSize)
	threes := bytes.Repeat([]byte{3}, chunkSize)
	fours := bytes.Repeat([]byte{4}, chunkSize)

	eds, err := ComputeExtendedDataSquare([][]byte{
		ones, twos,
		threes, fours,
	}, codec, NewDefaultTree)
	if err != nil {
		panic(err)
	}

	return eds
}

func TestCorruptedEdsReturnsErrByzantineData_UnorderedShares(t *testing.T) {
	chunkSize := 512
	namespaceSize := 1
	one := bytes.Repeat([]byte{1}, chunkSize)
	two := bytes.Repeat([]byte{2}, chunkSize)
	three := bytes.Repeat([]byte{3}, chunkSize)
	chunksValue := []int{1, 2, 3, 4}
	tests := []struct {
		name           string
		coords         [][]uint
		values         [][]byte
		wantErr        bool
		corruptedAxis  Axis
		corruptedIndex uint
	}{
		{
			name:    "no corruption",
			wantErr: false,
		},
		{
			// disturbs the order of chunks in the first row, erases the rest of the EDS
			name:          "rows with unordered chunks",
			wantErr:       true, // repair should error out during root construction
			corruptedAxis: Row,
			coords: [][]uint{
				{0, 0},
				{0, 1},
				{1, 0},
				{1, 1},
				{1, 2},
				{1, 3},
				{2, 0},
				{2, 1},
				{2, 2},
				{2, 3},
				{3, 0},
				{3, 1},
				{3, 2},
				{3, 3},
			},
			values: [][]byte{
				two, one,
				nil, nil, nil, nil,
				nil, nil, nil, nil,
				nil, nil, nil, nil,
			},
			corruptedIndex: 0,
		},
		{
			// disturbs the order of chunks in the first column, erases the rest of the EDS
			name:          "columns with unordered chunks",
			wantErr:       true, // repair should error out during root construction
			corruptedAxis: Col,
			coords: [][]uint{
				{0, 0},
				{0, 1},
				{0, 2},
				{0, 3},
				{1, 0},
				{1, 1},
				{1, 2},
				{1, 3},
				{2, 1},
				{2, 2},
				{2, 3},
				{3, 1},
				{3, 2},
				{3, 3},
			},
			values: [][]byte{
				three, nil, nil, nil,
				one, nil, nil, nil,
				nil, nil, nil,
				nil, nil, nil,
			},
			corruptedIndex: 0,
		},
	}

	codec := NewLeoRSCodec()

	// create a DA header
	eds := createTestEdsWithNMT(t, codec, chunkSize, namespaceSize, 1, 2, 3, 4)
	assert.NotNil(t, eds)
	dAHeaderRoots, err := eds.getRowRoots()
	assert.NoError(t, err)

	dAHeaderCols, err := eds.getColRoots()
	assert.NoError(t, err)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create an EDS with the given chunks
			corruptEds := createTestEdsWithNMT(t, codec, chunkSize, namespaceSize, chunksValue...)
			assert.NotNil(t, corruptEds)
			// corrupt it by setting the values at the given coordinates
			for i, coords := range test.coords {
				x := coords[0]
				y := coords[1]
				corruptEds.setCell(x, y, test.values[i])
			}

			err = corruptEds.Repair(dAHeaderRoots, dAHeaderCols)
			assert.Equal(t, err != nil, test.wantErr)
			if test.wantErr {
				var byzErr *ErrByzantineData
				assert.ErrorAs(t, err, &byzErr)
				errors.As(err, &byzErr)
				assert.Equal(t, byzErr.Axis, test.corruptedAxis)
				assert.Equal(t, byzErr.Index, test.corruptedIndex)
			}
		})
	}
}

// createTestEdsWithNMT creates an extended data square with the given chunks and namespace size.
// Chunks are placed in row-major order.
// The first namespaceSize bytes of each chunk is treated as its namespace.
// Roots of the extended data square are computed using namespace merkle trees.
func createTestEdsWithNMT(t *testing.T, codec Codec, chunkSize, namespaceSize int, chunkVals ...int) *ExtendedDataSquare {
	// the first namespaceSize bytes of each chunk are the namespace
	assert.True(t, chunkSize > namespaceSize)

	// create chunks of chunkSize bytes
	chunks := make([][]byte, len(chunkVals))
	for i, v := range chunkVals {
		chunks[i] = bytes.Repeat([]byte{byte(v)}, chunkSize)
	}
	edsWidth := 4            // number of chunks per row/column in the extended data square
	odsWidth := edsWidth / 2 // number of chunks per row/column in the original data square

	eds, err := ComputeExtendedDataSquare(chunks, codec, newConstructor(uint64(odsWidth), nmt.NamespaceIDSize(namespaceSize)))
	require.NoError(t, err)

	return eds
}
