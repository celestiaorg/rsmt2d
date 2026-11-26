package rsmt2d

import (
	"bytes"
	"errors"
	"slices"
	"sort"
	"testing"

	"github.com/celestiaorg/nmt"
	"github.com/stretchr/testify/require"
)

// fuzzNamespaceSize is the namespace size used when a fuzz target selects the
// namespaced Merkle tree. Corruptions never touch the first fuzzNamespaceSize
// bytes of a share so that the namespace order of the original data square is
// preserved.
const fuzzNamespaceSize = 8

// fuzzAdversarialOdsWidth maps a fuzzer-chosen byte onto a small original data
// square width. Unlike fuzzOdsWidth it includes widths that are not a power of
// two.
func fuzzAdversarialOdsWidth(sel uint8) int {
	return []int{1, 2, 3, 4, 5, 8}[int(sel)%6]
}

// fuzzTreeFn returns either the default Merkle tree or a namespaced Merkle
// tree (as used by celestia-app and celestia-node) for an ODS of width k.
func fuzzTreeFn(useNMT bool, k int) TreeConstructorFn {
	if !useNMT {
		return NewDefaultTree
	}
	return newErasuredNamespacedMerkleTreeConstructor(uint64(k), nmt.NamespaceIDSize(fuzzNamespaceSize))
}

// fuzzSortedOds returns fuzzOds(k, seed) sorted by namespace so that it is a
// valid input for the namespaced Merkle tree.
func fuzzSortedOds(k int, seed int64) [][]byte {
	ods := fuzzOds(k, seed)
	sort.Slice(ods, func(i, j int) bool {
		return bytes.Compare(ods[i][:fuzzNamespaceSize], ods[j][:fuzzNamespaceSize]) < 0
	})
	return ods
}

// flipShareByte returns a copy of share with one byte outside the namespace
// changed. The result always differs from share.
func flipShareByte(share []byte, sel uint8) []byte {
	out := bytes.Clone(share)
	pos := fuzzNamespaceSize + int(sel)%(len(out)-fuzzNamespaceSize)
	out[pos] ^= sel | 0x01
	return out
}

// axisIndexFn returns a function mapping the ith share of the given axis to
// its index in a flattened square of the given width.
func axisIndexFn(width int, axis Axis, axisIdx int) func(i int) int {
	if axis == Row {
		return func(i int) int { return axisIdx*width + i }
	}
	return func(i int) int { return i*width + axisIdx }
}

// reencodeAxis overwrites the parity half of an axis with the encoding of its
// first half, so that the axis becomes a valid codeword.
func reencodeAxis(t *testing.T, codec Codec, square [][]byte, at func(int) int, k int) {
	t.Helper()
	half := make([][]byte, k)
	for i := range half {
		half[i] = square[at(i)]
	}
	parity, err := codec.Encode(half)
	require.NoError(t, err)
	for i := range k {
		square[at(k+i)] = parity[i]
	}
}

// corruptSquare applies the corruptions encoded in ops to the flattened EDS
// square in place. Each corruption takes four bytes: mode, two index bytes and
// a value byte. A malicious block producer is free to commit to any square, so
// the modes model the interesting shapes of a badly encoded square:
//
//	0: change one share
//	1: change a share in the first half of a row and re-encode the row, so the
//	   row stays a codeword and only columns break
//	2: as 1, for a column
//	3: copy one share over another
//	4: change one share of an axis, erase some other shares of that axis and
//	   overwrite them with what the codec decodes from the rest, so the axis
//	   is not a codeword yet decodes consistently from that subset
//	5: re-encode every column from its first half
//	6: re-encode every row from its first half
//
// Modes 5 and 6 let a sequence of corruptions build squares where every
// column (or row) is a codeword but some row (or column) is not.
func corruptSquare(t *testing.T, codec Codec, square [][]byte, width int, ops []byte) {
	t.Helper()
	const opSize, maxOps, modes = 4, 4, 7
	k := width / 2
	for n := 0; n+opSize <= len(ops) && n/opSize < maxOps; n += opSize {
		mode, idx, val := ops[n]%modes, int(ops[n+1])<<8|int(ops[n+2]), ops[n+3]
		axis := Axis(idx & 1)
		switch mode {
		case 0:
			cell := idx % len(square)
			square[cell] = flipShareByte(square[cell], val)
		case 1, 2:
			at := axisIndexFn(width, Axis(mode-1), (idx>>1)%width)
			target := int(val) % k
			square[at(target)] = flipShareByte(square[at(target)], val)
			reencodeAxis(t, codec, square, at, k)
		case 3:
			square[idx%len(square)] = bytes.Clone(square[int(val)%len(square)])
		case 4:
			at := axisIndexFn(width, axis, (idx>>1)%width)
			target := int(val) % width
			square[at(target)] = flipShareByte(square[at(target)], val)
			shares := make([][]byte, width)
			for i := range shares {
				// Erase the shares selected by the upper index bits, never the
				// changed share itself.
				if i == target || (idx>>(8+i%8))&1 == 0 {
					shares[i] = bytes.Clone(square[at(i)])
				}
			}
			decoded, err := codec.Decode(shares)
			if err != nil {
				continue // too few shares left to decode
			}
			for i := range shares {
				square[at(i)] = decoded[i]
			}
		case 5, 6:
			for i := range width {
				reencodeAxis(t, codec, square, axisIndexFn(width, Axis(6-mode), i), k)
			}
		}
	}
}

// axisOf returns the shares of the given axis of a flattened square.
func axisOf(square [][]byte, width int, axis Axis, idx int) [][]byte {
	out := make([][]byte, width)
	for i := range out {
		if axis == Row {
			out[i] = square[idx*width+i]
		} else {
			out[i] = square[i*width+idx]
		}
	}
	return out
}

// computeAxisRoot computes the Merkle root of a complete axis with treeFn.
func computeAxisRoot(treeFn TreeConstructorFn, axis Axis, idx int, shares [][]byte) ([]byte, error) {
	tree := treeFn(axis, uint(idx))
	for _, s := range shares {
		if err := tree.Push(s); err != nil {
			releaseTree(tree)
			return nil, err
		}
	}
	return tree.Root()
}

// inconsistentAxes reports, per axis, which rows or columns of a flattened
// square are not valid codewords.
func inconsistentAxes(t *testing.T, codec Codec, square [][]byte, width int) map[Axis][]bool {
	t.Helper()
	bad := map[Axis][]bool{Row: make([]bool, width), Col: make([]bool, width)}
	for _, axis := range []Axis{Row, Col} {
		for i := range width {
			bad[axis][i] = !isCodeword(t, codec, axisOf(square, width, axis, i))
		}
	}
	return bad
}

// tamperRoot changes the root selected by tamperSel (zero selects none) and
// marks that axis as inconsistent in bad.
func tamperRoot(rowRoots, colRoots [][]byte, tamperSel uint16, bad map[Axis][]bool) {
	if tamperSel == 0 {
		return
	}
	width := len(rowRoots)
	idx := int(tamperSel-1) % (2 * width)
	roots, axis := rowRoots, Row
	if idx >= width {
		roots, axis, idx = colRoots, Col, idx-width
	}
	tampered := bytes.Clone(roots[idx])
	tampered[int(tamperSel)%len(tampered)] ^= 0x01
	roots[idx] = tampered
	bad[axis][idx] = true
}

// fuzzMaskedEds returns a new, otherwise empty EDS holding copies of the
// shares of the flattened square selected by mask.
func fuzzMaskedEds(t *testing.T, codec Codec, treeFn TreeConstructorFn, square [][]byte, mask []byte) *ExtendedDataSquare {
	t.Helper()
	width := getWidth(square)
	eds, err := NewExtendedDataSquare(codec, treeFn, uint(width), fuzzShareSize)
	require.NoError(t, err)
	for i := range square {
		if maskBit(mask, i) {
			require.NoError(t, eds.SetCell(uint(i/width), uint(i%width), bytes.Clone(square[i])))
		}
	}
	return eds
}

// requireValidFraudProof checks that byzErr converts into a bad encoding
// fraud proof the way celestia-node builds one: every reported share must be
// the committed share at that position (so it is provable against the
// committed roots), at least k shares must be reported, and the axis decoded
// from the first k of them must not match root.
func requireValidFraudProof(t *testing.T, codec Codec, treeFn TreeConstructorFn, byzErr *ErrByzantineData, committedAxis [][]byte, root []byte) {
	t.Helper()
	width := len(committedAxis)
	k := width / 2
	require.Len(t, byzErr.Shares, width)
	proof := make([][]byte, width)
	present := 0
	for i, s := range byzErr.Shares {
		if s == nil {
			continue
		}
		require.Equal(t, committedAxis[i], s, "reported share %d is not a committed share", i)
		if present < k {
			proof[i] = bytes.Clone(s)
		}
		present++
	}
	require.GreaterOrEqual(t, present, k, "too few shares to build a fraud proof")

	decoded, err := codec.Decode(proof)
	require.NoError(t, err)
	got, err := computeAxisRoot(treeFn, byzErr.Axis, int(byzErr.Index), decoded)
	if err == nil {
		require.NotEqual(t, root, got, "fraud proof for %s %d would not verify", byzErr.Axis, byzErr.Index)
	}
}

// FuzzRepairByzantineSquare checks the soundness of Repair against a malicious
// block producer. The producer commits (through row and column roots) to an
// arbitrary square derived from an honest EDS by the corruptions in ops, and
// may additionally publish one tampered root. A light node then repairs from
// an arbitrary subset of the committed shares. For every outcome:
//
//   - nil: the committed square is a valid EDS, every root is untampered and
//     the repaired square equals the committed square.
//   - ErrByzantineData: the reported axis really is inconsistent (its
//     committed shares are not a codeword, or its root was tampered), every
//     reported share is a committed share, at least k shares are reported and
//     decoding the first k of them yields an axis whose root differs from the
//     committed root, i.e. the error converts into a valid bad encoding fraud
//     proof the way celestia-node builds one.
//   - ErrUnrepairableDataSquare: only if no full axis set had k shares each.
//
// With no effective corruption this is also a completeness check over ODS
// widths that are not a power of two and over the namespaced Merkle tree.
func FuzzRepairByzantineSquare(f *testing.F) {
	f.Add(int64(1), uint8(1), false, []byte{}, uint16(0), []byte{0xff})
	f.Add(int64(2), uint8(2), false, []byte{0, 0, 3, 7}, uint16(0), []byte{0xaa, 0x55})
	f.Add(int64(3), uint8(3), true, []byte{1, 0, 1, 9}, uint16(0), []byte{0x0f, 0xf0, 0x3c})
	f.Add(int64(4), uint8(1), false, []byte{2, 0, 2, 1}, uint16(0), []byte{0x33})
	f.Add(int64(5), uint8(4), true, []byte{0, 0, 9, 4, 1, 0, 5, 3}, uint16(0), []byte{0xff, 0x00})
	f.Add(int64(6), uint8(2), false, []byte{}, uint16(3), []byte{0xff, 0xff})
	f.Add(int64(7), uint8(0), true, []byte{3, 0, 1, 0}, uint16(1), []byte{})
	f.Add(int64(8), uint8(5), true, []byte{2, 0, 6, 200}, uint16(0), []byte{0x5a, 0xa5, 0x0f})
	// A row that is not a codeword but decodes consistently from the shares
	// available to the light node, while every column is a codeword.
	f.Add(int64(9), uint8(1), false, []byte{4, 0x08, 0x00, 2, 5, 0, 0, 0}, uint16(0), []byte{0x37, 0x04})
	f.Add(int64(10), uint8(1), true, []byte{4, 0x08, 0x00, 2, 5, 0, 0, 0}, uint16(0), []byte{0x37, 0x04})

	f.Fuzz(func(t *testing.T, seed int64, kSel uint8, useNMT bool, ops []byte, tamperSel uint16, mask []byte) {
		codec := NewLeoRSCodec()
		k := fuzzAdversarialOdsWidth(kSel)
		treeFn := fuzzTreeFn(useNMT, k)

		honest, err := ComputeExtendedDataSquare(fuzzSortedOds(k, seed), codec, treeFn)
		require.NoError(t, err)
		width := int(honest.Width())
		committed := honest.Flattened()
		corruptSquare(t, codec, committed, width, ops)

		// The producer must be able to compute roots over the square it
		// commits to; with the namespaced tree some corruptions break the
		// namespace order and cannot be committed to.
		committedEds, err := ImportExtendedDataSquare(committed, codec, treeFn)
		require.NoError(t, err)
		rowRoots, err := committedEds.RowRoots()
		if err != nil {
			return
		}
		colRoots, err := committedEds.ColRoots()
		require.NoError(t, err)

		bad := inconsistentAxes(t, codec, committed, width)
		tamperRoot(rowRoots, colRoots, tamperSel, bad)
		anyBad := slices.Contains(bad[Row], true) || slices.Contains(bad[Col], true)

		partial := fuzzMaskedEds(t, codec, treeFn, committed, mask)
		guaranteed := hasAxisWithEnoughShares(partial, k)

		err = partial.Repair(rowRoots, colRoots)
		var byzErr *ErrByzantineData
		switch {
		case err == nil:
			require.False(t, anyBad, "Repair succeeded on an inconsistent square")
			require.Equal(t, committed, partial.Flattened(), "repaired square differs from committed square")
			got, err := partial.RowRoots()
			require.NoError(t, err)
			require.Equal(t, rowRoots, got)
			got, err = partial.ColRoots()
			require.NoError(t, err)
			require.Equal(t, colRoots, got)
		case errors.As(err, &byzErr):
			idx := int(byzErr.Index)
			require.Less(t, idx, width)
			require.True(t, bad[byzErr.Axis][idx], "%s %d reported as byzantine but it is consistent", byzErr.Axis, idx)
			root := rowRoots[idx]
			if byzErr.Axis == Col {
				root = colRoots[idx]
			}
			requireValidFraudProof(t, codec, treeFn, byzErr, axisOf(committed, width, byzErr.Axis, idx), root)
		default:
			require.ErrorIs(t, err, ErrUnrepairableDataSquare)
			require.False(t, guaranteed, "square with >= k shares in every row or column must not be unrepairable")
		}
	})
}

// fuzzShares builds n shares of the given size from raw. Shares selected by
// nilMask are nil, and the share at raggedIdx (if any) is one byte longer.
func fuzzShares(n, size, raggedIdx int, nilMask, raw []byte) [][]byte {
	shares := make([][]byte, n)
	for i := range shares {
		if maskBit(nilMask, i) {
			continue
		}
		l := size
		if i == raggedIdx {
			l++
		}
		shares[i] = make([]byte, l)
		for j := range shares[i] {
			if len(raw) > 0 {
				shares[i][j] = raw[(i*l+j)%len(raw)]
			}
		}
	}
	return shares
}

// FuzzLeoRSCodecArbitraryInput feeds malformed share layouts into the
// Leopard codec: any count (including zero and odd counts), invalid and mixed
// share sizes, and arbitrary nil patterns. Encode and Decode must return an
// error rather than panic. When Decode succeeds it must keep every present
// share unchanged, fill every missing share with a share of the same size, and
// when exactly half of the shares are present the result must be the unique
// codeword through them.
func FuzzLeoRSCodecArbitraryInput(f *testing.F) {
	f.Add(uint8(4), uint8(3), uint8(255), []byte{0x05}, []byte{1, 2, 3}, true)
	f.Add(uint8(4), uint8(3), uint8(255), []byte{}, []byte{1, 2, 3}, false)
	f.Add(uint8(0), uint8(3), uint8(255), []byte{}, []byte{}, true)
	f.Add(uint8(0), uint8(3), uint8(255), []byte{}, []byte{}, false)
	f.Add(uint8(1), uint8(3), uint8(255), []byte{}, []byte{7}, true)
	f.Add(uint8(5), uint8(3), uint8(255), []byte{0x03}, []byte{9}, true)    // odd count
	f.Add(uint8(8), uint8(3), uint8(2), []byte{0x11}, []byte{4, 5}, true)   // ragged share
	f.Add(uint8(8), uint8(2), uint8(255), []byte{0x11}, []byte{4, 5}, true) // 63-byte shares
	f.Add(uint8(8), uint8(0), uint8(255), []byte{0x11}, []byte{}, true)     // empty shares
	f.Add(uint8(6), uint8(4), uint8(255), []byte{0x07}, []byte{0xff}, false)
	f.Add(uint8(4), uint8(3), uint8(255), []byte{0xff}, []byte{1}, false) // all nil

	f.Fuzz(func(t *testing.T, n, sizeSel, raggedIdx uint8, nilMask, raw []byte, decode bool) {
		codec := NewLeoRSCodec()
		count := int(n) % 34
		size := []int{0, 1, 63, 64, 128, 192}[int(sizeSel)%6]
		shares := fuzzShares(count, size, int(raggedIdx), nilMask, raw)

		if !decode {
			parity, err := codec.Encode(shares)
			if err != nil {
				return
			}
			require.Len(t, parity, count)
			for _, p := range parity {
				require.Len(t, p, len(shares[0]))
			}
			return
		}

		input := deepCopy(shares)
		out, err := codec.Decode(shares)
		if err != nil {
			return
		}
		require.Len(t, out, count)
		present, shareLen := 0, -1
		for i, s := range input {
			if s == nil {
				continue
			}
			present++
			shareLen = len(s)
			require.Equal(t, s, out[i], "Decode changed present share %d", i)
		}
		for i, s := range out {
			require.NotNil(t, s, "Decode left share %d missing", i)
			require.Len(t, s, shareLen, "Decode returned share %d of a different size", i)
		}
		if count%2 == 0 && present == count/2 {
			require.True(t, isCodeword(t, codec, out), "Decode from exactly k shares must return a codeword")
		}
	})
}

// fuzzRoots returns n roots of rootLen bytes derived from raw.
func fuzzRoots(n, rootLen int, raw []byte, salt int) [][]byte {
	roots := make([][]byte, n)
	for i := range roots {
		roots[i] = make([]byte, rootLen)
		for j := range roots[i] {
			if len(raw) > 0 {
				roots[i][j] = raw[(salt+i*rootLen+j)%len(raw)] + byte(salt)
			}
		}
	}
	return roots
}

// requireConsistentViews checks that every read accessor of eds agrees with
// want, the expected flattened contents of the square.
func requireConsistentViews(t *testing.T, eds *ExtendedDataSquare, want [][]byte) {
	t.Helper()
	width := int(eds.Width())
	require.Equal(t, len(want), width*width)
	require.Equal(t, want, eds.Flattened())
	k := width / 2
	ods := eds.FlattenedODS()
	require.Len(t, ods, k*k)
	for r := range width {
		row, col := eds.Row(uint(r)), eds.Col(uint(r))
		for c := range width {
			require.Equal(t, want[r*width+c], row[c])
			require.Equal(t, want[c*width+r], col[c])
			require.Equal(t, want[r*width+c], eds.GetCell(uint(r), uint(c)))
			if r < k && c < k {
				require.Equal(t, want[r*width+c], ods[r*k+c])
			}
		}
	}
	require.True(t, eds.Equals(eds))
}

// FuzzImportAndRepairArbitrary feeds attacker-shaped squares and roots into
// ImportExtendedDataSquare and Repair: any share count, widths that are zero,
// odd or not a power of two, invalid or mixed share sizes, any nil pattern,
// and any number and length of row and column roots, with both tree
// implementations. Nothing may panic. Import must reject inputs that violate
// its preconditions, every accessor of an imported square must agree with the
// input, and Repair must either reject the roots or return a square whose
// roots are exactly the supplied roots.
func FuzzImportAndRepairArbitrary(f *testing.F) {
	f.Add(uint8(4), uint8(2), uint8(255), []byte{0x01}, []byte{1, 2, 3}, uint8(4), uint8(4), uint8(32), false)
	f.Add(uint8(4), uint8(2), uint8(255), []byte{0x01}, []byte{1, 2, 3}, uint8(4), uint8(4), uint8(48), true)
	f.Add(uint8(5), uint8(2), uint8(255), []byte{0x21, 0x84}, []byte{9}, uint8(6), uint8(6), uint8(32), false) // width 6
	f.Add(uint8(3), uint8(2), uint8(255), []byte{}, []byte{9}, uint8(3), uint8(3), uint8(32), false)           // odd width
	f.Add(uint8(0), uint8(2), uint8(255), []byte{}, []byte{}, uint8(0), uint8(0), uint8(0), false)             // empty
	f.Add(uint8(2), uint8(1), uint8(255), []byte{}, []byte{5}, uint8(2), uint8(2), uint8(32), false)           // 63-byte shares
	f.Add(uint8(2), uint8(2), uint8(1), []byte{}, []byte{5}, uint8(2), uint8(2), uint8(32), false)             // ragged share
	f.Add(uint8(0x82), uint8(2), uint8(255), []byte{}, []byte{5}, uint8(2), uint8(2), uint8(32), false)        // non-square count
	f.Add(uint8(4), uint8(2), uint8(255), []byte{0xff}, []byte{5}, uint8(4), uint8(4), uint8(32), true)        // all nil
	f.Add(uint8(6), uint8(3), uint8(255), []byte{0x10}, []byte{0, 7}, uint8(10), uint8(9), uint8(90), true)

	f.Fuzz(func(t *testing.T, widthSel, sizeSel, raggedIdx uint8, nilMask, raw []byte, nRows, nCols, rootLen uint8, useNMT bool) {
		codec := NewLeoRSCodec()
		width := []int{0, 1, 2, 3, 4, 6, 8, 10}[int(widthSel)%8]
		count := width * width
		if widthSel&0x80 != 0 {
			count++
		}
		size := []int{0, 63, 64, 128}[int(sizeSel)%4]
		shares := fuzzShares(count, size, int(raggedIdx), nilMask, raw)
		treeFn := NewDefaultTree
		if width >= 2 {
			treeFn = fuzzTreeFn(useNMT, width/2)
		}

		eds, err := ImportExtendedDataSquare(shares, codec, treeFn)
		if err != nil {
			return
		}
		// A successful import implies every precondition held.
		require.Equal(t, width*width, count)
		require.Positive(t, width)
		require.Zero(t, width%2)
		shareLen := getShareSize(shares)
		require.NoError(t, codec.ValidateChunkSize(shareLen))
		for _, s := range shares {
			require.True(t, s == nil || len(s) == shareLen, "accepted uneven shares")
		}
		require.Equal(t, uint(width), eds.Width())
		requireConsistentViews(t, eds, deepCopy(shares))

		rowRoots := fuzzRoots(int(nRows)%12, int(rootLen)%96, raw, 1)
		colRoots := fuzzRoots(int(nCols)%12, int(rootLen)%96, raw, 2)
		err = eds.Repair(rowRoots, colRoots)
		if len(rowRoots) != width || len(colRoots) != width {
			require.ErrorIs(t, err, ErrInvalidRootsLength)
			return
		}
		var byzErr *ErrByzantineData
		switch {
		case err == nil:
			gotRows, err := eds.RowRoots()
			require.NoError(t, err)
			require.Equal(t, rowRoots, gotRows)
			gotCols, err := eds.ColRoots()
			require.NoError(t, err)
			require.Equal(t, colRoots, gotCols)
		case errors.As(err, &byzErr):
			require.Less(t, int(byzErr.Index), width)
			require.Len(t, byzErr.Shares, width)
		default:
			require.ErrorIs(t, err, ErrUnrepairableDataSquare)
		}
	})
}

// mutationHarness drives an EDS through public mutation APIs while tracking
// the contents it is expected to hold.
type mutationHarness struct {
	t         *testing.T
	codec     Codec
	treeFn    TreeConstructorFn
	eds       *ExtendedDataSquare
	committed [][]byte // the honest square
	model     [][]byte // expected contents of eds
	rowRoots  [][]byte // honest row roots
	colRoots  [][]byte // honest column roots
}

// setCell calls SetCell with the committed share of cell, or a corrupted copy
// of it, and checks that SetCell succeeds exactly when the cell is empty.
func (h *mutationHarness) setCell(cell int, corrupt bool, sel uint8) {
	width := int(h.eds.Width())
	share := bytes.Clone(h.committed[cell])
	if corrupt {
		share = flipShareByte(share, sel)
	}
	err := h.eds.SetCell(uint(cell/width), uint(cell%width), bytes.Clone(share))
	if h.model[cell] != nil {
		require.Error(h.t, err, "SetCell overwrote a present share")
		return
	}
	require.NoError(h.t, err)
	h.model[cell] = share
}

// checkRoots checks that Roots fails on an incomplete square and otherwise
// equals the roots of a freshly imported copy of the expected contents.
func (h *mutationHarness) checkRoots() {
	got, err := h.eds.Roots()
	if !isComplete(h.model) {
		require.Error(h.t, err, "roots of an incomplete square")
		return
	}
	fresh, err2 := ImportExtendedDataSquare(deepCopy(h.model), h.codec, h.treeFn)
	require.NoError(h.t, err2)
	want, err2 := fresh.Roots()
	if err2 != nil {
		require.Error(h.t, err, "roots of a square whose roots cannot be computed")
		return
	}
	require.NoError(h.t, err)
	require.Equal(h.t, want, got, "cached roots differ from a fresh import")
}

// repair calls Repair with the honest roots. It must succeed only if no
// corrupted share is present, must not report honest shares as byzantine,
// and may only fill empty cells with their committed shares.
func (h *mutationHarness) repair() {
	corrupted := false
	for i, s := range h.model {
		corrupted = corrupted || (s != nil && !bytes.Equal(s, h.committed[i]))
	}
	err := h.eds.Repair(h.rowRoots, h.colRoots)
	switch {
	case corrupted:
		require.Error(h.t, err, "Repair succeeded with a corrupted share present")
	case err != nil:
		require.ErrorIs(h.t, err, ErrUnrepairableDataSquare, "honest shares reported as byzantine")
	}
	after := h.eds.Flattened()
	for i := range h.model {
		if h.model[i] == nil && after[i] != nil {
			require.Equal(h.t, h.committed[i], after[i], "Repair filled cell %d with a share that is not committed", i)
		}
	}
	h.model = after
	if err == nil {
		require.Equal(h.t, h.committed, h.model)
	}
}

// FuzzEDSMutationAPIs drives an EDS through an arbitrary sequence of SetCell
// (with honest, corrupted and wrongly sized shares), root queries, Repair and
// reads, starting from an arbitrary subset of an honest square. It checks
// differentially against a model of the expected contents:
//
//   - SetCell succeeds exactly on empty cells with correctly sized shares;
//   - every read accessor agrees with the model;
//   - root queries fail on an incomplete square, and on a complete square
//     equal the roots of a freshly imported copy (so cached roots are never
//     stale after a mutation);
//   - Repair with the honest roots succeeds only if no corrupted share is
//     present, and only ever fills cells with the honest shares.
func FuzzEDSMutationAPIs(f *testing.F) {
	f.Add(int64(1), uint8(1), false, []byte{0xff, 0x0f}, []byte{3, 0, 0, 5, 4, 0, 3, 0, 5, 7})
	f.Add(int64(2), uint8(2), true, []byte{0x5a}, []byte{1, 3, 4, 0, 3, 0, 0, 3, 3, 0})
	f.Add(int64(3), uint8(0), false, []byte{0x0e}, []byte{0, 0, 3, 0, 2, 1, 4, 0, 3, 0})
	f.Add(int64(4), uint8(3), true, []byte{}, []byte{4, 0, 0, 1, 0, 2, 3, 0, 5, 9})
	f.Add(int64(5), uint8(1), false, []byte{0xff, 0xff}, []byte{3, 0, 1, 0, 4, 0, 3, 0})

	f.Fuzz(func(t *testing.T, seed int64, kSel uint8, useNMT bool, mask, ops []byte) {
		codec := NewLeoRSCodec()
		k := fuzzAdversarialOdsWidth(kSel)
		treeFn := fuzzTreeFn(useNMT, k)
		honest, err := ComputeExtendedDataSquare(fuzzSortedOds(k, seed), codec, treeFn)
		require.NoError(t, err)
		h := &mutationHarness{t: t, codec: codec, treeFn: treeFn, committed: honest.Flattened()}
		h.rowRoots, err = honest.RowRoots()
		require.NoError(t, err)
		h.colRoots, err = honest.ColRoots()
		require.NoError(t, err)
		h.eds = fuzzMaskedEds(t, codec, treeFn, h.committed, mask)
		h.model = h.eds.Flattened()

		const maxOps = 32
		for n := 0; n+1 < len(ops) && n/2 < maxOps; n += 2 {
			op, arg := ops[n]%6, ops[n+1]
			cell := int(arg) % len(h.model)
			switch op {
			case 0, 1:
				h.setCell(cell, op == 1, arg)
			case 2:
				width := h.eds.Width()
				wrongSize := make([]byte, fuzzShareSize+int(arg)%2*128-64)
				require.Error(t, h.eds.SetCell(uint(cell)/width, uint(cell)%width, wrongSize))
			case 3:
				h.checkRoots()
			case 4:
				h.repair()
			case 5:
				requireConsistentViews(t, h.eds, h.model)
			}
		}
		requireConsistentViews(t, h.eds, h.model)
		h.checkRoots()
	})
}
