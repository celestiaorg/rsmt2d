package rsmt2d

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// fuzzShareSize is a small, codec-valid share size that keeps each fuzz
// iteration cheap. The Leopard codec requires a multiple of 64 bytes.
const fuzzShareSize = 64

// fuzzOdsWidth maps a fuzzer-chosen byte onto a small original data square
// (ODS) width so that iterations stay fast.
func fuzzOdsWidth(sel uint8) int {
	return []int{1, 2, 4, 8}[int(sel)%4]
}

// fuzzOds returns a k x k ODS whose shares are derived deterministically from
// seed.
func fuzzOds(k int, seed int64) [][]byte {
	rng := rand.New(rand.NewSource(seed))
	ods := make([][]byte, k*k)
	for i := range ods {
		ods[i] = make([]byte, fuzzShareSize)
		rng.Read(ods[i])
	}
	return ods
}

// maskBit reports whether bit i of mask is set. mask is treated as cyclic so
// that a short mask still selects cells across the whole square. An empty mask
// selects nothing.
func maskBit(mask []byte, i int) bool {
	if len(mask) == 0 {
		return false
	}
	return mask[(i/8)%len(mask)]>>(i%8)&1 == 1
}

// fuzzPartialEds returns a new EDS of the same width as src that is populated
// with the cells of src selected by mask. Every cell in always is populated
// regardless of mask.
func fuzzPartialEds(t *testing.T, codec Codec, src *ExtendedDataSquare, mask []byte, always ...int) *ExtendedDataSquare {
	t.Helper()
	width := int(src.Width())
	partial, err := NewExtendedDataSquare(codec, NewDefaultTree, uint(width), fuzzShareSize)
	require.NoError(t, err)
	for i := range width * width {
		set := maskBit(mask, i)
		for _, a := range always {
			set = set || a == i
		}
		if !set {
			continue
		}
		row, col := uint(i/width), uint(i%width)
		require.NoError(t, partial.SetCell(row, col, src.GetCell(row, col)))
	}
	return partial
}

// hasAxisWithEnoughShares reports whether every row (or every column) of eds
// has at least k non-nil shares. When true, the crossword solver is guaranteed
// to repair the square in a single pass, so Repair must succeed.
func hasAxisWithEnoughShares(eds *ExtendedDataSquare, k int) bool {
	width := int(eds.Width())
	rowsOk, colsOk := true, true
	for i := range width {
		rowPresent, colPresent := 0, 0
		for j := range width {
			if eds.GetCell(uint(i), uint(j)) != nil {
				rowPresent++
			}
			if eds.GetCell(uint(j), uint(i)) != nil {
				colPresent++
			}
		}
		rowsOk = rowsOk && rowPresent >= k
		colsOk = colsOk && colPresent >= k
	}
	return rowsOk || colsOk
}

// FuzzRepair checks the data availability guarantee of Repair on an honestly
// encoded square: for any subset of available shares, Repair either
// reconstructs exactly the original square or reports
// ErrUnrepairableDataSquare. It must never panic and must never flag honest
// data as byzantine.
func FuzzRepair(f *testing.F) {
	f.Add(int64(1), uint8(1), []byte{0xff, 0xff})
	f.Add(int64(2), uint8(2), []byte{0x0f, 0xf0, 0x0f, 0xf0})
	f.Add(int64(3), uint8(3), []byte{0x01})
	f.Add(int64(4), uint8(0), []byte{})

	f.Fuzz(func(t *testing.T, seed int64, kSel uint8, mask []byte) {
		codec := NewLeoRSCodec()
		k := fuzzOdsWidth(kSel)
		original, err := ComputeExtendedDataSquare(fuzzOds(k, seed), codec, NewDefaultTree)
		require.NoError(t, err)
		rowRoots, err := original.RowRoots()
		require.NoError(t, err)
		colRoots, err := original.ColRoots()
		require.NoError(t, err)

		partial := fuzzPartialEds(t, codec, original, mask)
		guaranteed := hasAxisWithEnoughShares(partial, k)

		err = partial.Repair(rowRoots, colRoots)
		if err != nil {
			require.ErrorIs(t, err, ErrUnrepairableDataSquare, "honest data must never be byzantine")
			require.False(t, guaranteed, "square with >= k shares in every row or column must be repairable")
			return
		}

		require.True(t, partial.Equals(original), "repaired square differs from original")
		gotRowRoots, err := partial.RowRoots()
		require.NoError(t, err)
		require.Equal(t, rowRoots, gotRowRoots)
		gotColRoots, err := partial.ColRoots()
		require.NoError(t, err)
		require.Equal(t, colRoots, gotColRoots)

		// Repairing a complete square is a no-op.
		require.NoError(t, partial.Repair(rowRoots, colRoots))
	})
}

// FuzzRepairCorrupted checks the fraud proof guarantee of Repair: when a
// single share of an otherwise honest square is corrupted and the roots are
// committed over the corrupted square, Repair from any subset of shares that
// includes the corrupted share must never succeed. If it detects the
// corruption, the returned ErrByzantineData must point at the corrupted row or
// column.
func FuzzRepairCorrupted(f *testing.F) {
	f.Add(int64(1), uint8(1), uint16(0), []byte{0xff, 0xff})
	f.Add(int64(2), uint8(2), uint16(5), []byte{0xf0, 0x0f, 0x33, 0xcc})
	f.Add(int64(3), uint8(3), uint16(200), []byte{0x01, 0x80})
	f.Add(int64(4), uint8(0), uint16(3), []byte{})

	f.Fuzz(func(t *testing.T, seed int64, kSel uint8, corruptSel uint16, mask []byte) {
		codec := NewLeoRSCodec()
		k := fuzzOdsWidth(kSel)
		original, err := ComputeExtendedDataSquare(fuzzOds(k, seed), codec, NewDefaultTree)
		require.NoError(t, err)

		width := int(original.Width())
		flattened := original.Flattened()
		corruptIdx := int(corruptSel) % len(flattened)
		corruptRow, corruptCol := uint(corruptIdx/width), uint(corruptIdx%width)

		// Replace the chosen share with a pseudo-random share that is
		// guaranteed to differ from the original.
		rng := rand.New(rand.NewSource(seed + 1))
		corruptShare := make([]byte, fuzzShareSize)
		rng.Read(corruptShare)
		if bytes.Equal(corruptShare, flattened[corruptIdx]) {
			corruptShare[0] ^= 0x01
		}
		flattened[corruptIdx] = corruptShare

		corrupted, err := ImportExtendedDataSquare(flattened, codec, NewDefaultTree)
		require.NoError(t, err)
		rowRoots, err := corrupted.RowRoots()
		require.NoError(t, err)
		colRoots, err := corrupted.ColRoots()
		require.NoError(t, err)

		partial := fuzzPartialEds(t, codec, corrupted, mask, corruptIdx)
		err = partial.Repair(rowRoots, colRoots)
		require.Error(t, err, "Repair must not succeed on a square containing a corrupted share")

		var byzErr *ErrByzantineData
		if !errors.As(err, &byzErr) {
			require.ErrorIs(t, err, ErrUnrepairableDataSquare)
			return
		}
		switch byzErr.Axis {
		case Row:
			require.Equal(t, corruptRow, byzErr.Index, "byzantine row must be the corrupted row")
		case Col:
			require.Equal(t, corruptCol, byzErr.Index, "byzantine col must be the corrupted col")
		default:
			t.Fatalf("unexpected axis %d", byzErr.Axis)
		}
		require.Len(t, byzErr.Shares, width)
	})
}

// FuzzComputeAndImportExtendedDataSquare feeds arbitrary share layouts into
// ComputeExtendedDataSquare. It must never panic, must reject inputs that
// violate its documented preconditions, and every square it does produce must
// survive a Flattened -> ImportExtendedDataSquare round trip.
func FuzzComputeAndImportExtendedDataSquare(f *testing.F) {
	f.Add(uint8(4), uint8(64), uint8(255), []byte("some data"))
	f.Add(uint8(1), uint8(64), uint8(255), []byte{})
	f.Add(uint8(3), uint8(64), uint8(255), []byte{1, 2, 3}) // not a square number
	f.Add(uint8(4), uint8(65), uint8(255), []byte{1, 2, 3}) // invalid share size
	f.Add(uint8(4), uint8(64), uint8(2), []byte{1, 2, 3})   // uneven shares
	f.Add(uint8(0), uint8(0), uint8(255), []byte{})         // empty square
	f.Add(uint8(4), uint8(0), uint8(255), []byte{})         // zero-size shares
	f.Add(uint8(16), uint8(128), uint8(255), []byte{0xaa})  // larger valid square

	f.Fuzz(func(t *testing.T, n uint8, shareSize uint8, unevenIdx uint8, raw []byte) {
		codec := NewLeoRSCodec()
		data := make([][]byte, n)
		for i := range data {
			size := int(shareSize)
			if int(unevenIdx) == i {
				size++
			}
			data[i] = make([]byte, size)
			for j := range data[i] {
				if len(raw) > 0 {
					data[i][j] = raw[(i*size+j)%len(raw)]
				}
			}
		}

		eds, err := ComputeExtendedDataSquare(data, codec, NewDefaultTree)
		if err != nil {
			return
		}

		// A successful result implies every documented precondition held.
		odsWidth := getWidth(data)
		require.Equal(t, len(data), odsWidth*odsWidth, "accepted a non-square number of shares")
		require.Zero(t, getShareSize(data)%64, "accepted an invalid share size")
		if len(data) > 1 {
			require.GreaterOrEqual(t, int(unevenIdx), len(data), "accepted uneven shares")
		}
		require.Equal(t, uint(2*odsWidth), eds.Width())
		require.Equal(t, data, eds.FlattenedODS())

		rowRoots, err := eds.RowRoots()
		require.NoError(t, err)
		colRoots, err := eds.ColRoots()
		require.NoError(t, err)

		imported, err := ImportExtendedDataSquare(eds.Flattened(), codec, NewDefaultTree)
		require.NoError(t, err)
		require.True(t, imported.Equals(eds))
		importedRowRoots, err := imported.RowRoots()
		require.NoError(t, err)
		require.Equal(t, rowRoots, importedRowRoots)
		importedColRoots, err := imported.ColRoots()
		require.NoError(t, err)
		require.Equal(t, colRoots, importedColRoots)

		// A freshly computed square is a valid encoding, so Repair must be a
		// no-op.
		require.NoError(t, imported.Repair(rowRoots, colRoots))
	})
}

// FuzzUnmarshalJSON feeds arbitrary bytes into ExtendedDataSquare.UnmarshalJSON.
// It must never panic, and any square it accepts must round trip through
// MarshalJSON.
func FuzzUnmarshalJSON(f *testing.F) {
	codec := NewLeoRSCodec()
	valid, err := ComputeExtendedDataSquare(fuzzOds(2, 1), codec, NewDefaultTree)
	require.NoError(f, err)
	validJSON, err := json.Marshal(valid)
	require.NoError(f, err)

	f.Add(validJSON)
	f.Add([]byte(`{"data_square":[],"codec":"Leopard"}`))
	f.Add([]byte(`{"data_square":[],"codec":"unknown"}`))
	f.Add([]byte(`{"data_square":[null,null,null,null],"codec":"Leopard"}`))
	f.Add([]byte(`{"data_square":["AA==","AA==","AA=="],"codec":"Leopard"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`not json`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var eds ExtendedDataSquare
		if err := json.Unmarshal(data, &eds); err != nil {
			return
		}

		out, err := json.Marshal(&eds)
		require.NoError(t, err)
		var again ExtendedDataSquare
		require.NoError(t, json.Unmarshal(out, &again))
		require.True(t, again.Equals(&eds))
	})
}

// FuzzLeoRSCodec checks the Reed-Solomon codec directly: parity computed by
// Encode must be recoverable by Decode from any k of the 2k codeword shares,
// and Decode must fail (not panic) when fewer than k shares are available.
func FuzzLeoRSCodec(f *testing.F) {
	f.Add(int64(1), uint8(1), uint8(0), []byte{0xff})
	f.Add(int64(2), uint8(2), uint8(1), []byte{0x0f})
	f.Add(int64(3), uint8(3), uint8(2), []byte{0x01, 0x00})
	f.Add(int64(4), uint8(0), uint8(3), []byte{})

	f.Fuzz(func(t *testing.T, seed int64, kSel uint8, sizeSel uint8, mask []byte) {
		codec := NewLeoRSCodec()
		k := []int{1, 2, 4, 8, 16}[int(kSel)%5]
		shareSize := 64 * (1 + int(sizeSel)%4)

		rng := rand.New(rand.NewSource(seed))
		data := make([][]byte, k)
		for i := range data {
			data[i] = make([]byte, shareSize)
			rng.Read(data[i])
		}

		parity, err := codec.Encode(data)
		require.NoError(t, err)
		require.Len(t, parity, k)

		codeword := make([][]byte, 0, 2*k)
		codeword = append(codeword, data...)
		codeword = append(codeword, parity...)

		erased := make([][]byte, 2*k)
		present := 0
		for i := range erased {
			if maskBit(mask, i) {
				erased[i] = bytes.Clone(codeword[i])
				present++
			}
		}

		decoded, err := codec.Decode(erased)
		if present < k {
			require.Error(t, err, "Decode must fail with fewer than k shares")
			return
		}
		require.NoError(t, err)
		require.Equal(t, codeword, decoded)
	})
}
