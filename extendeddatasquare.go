// Package rsmt2d implements the two dimensional Reed-Solomon merkle tree data availability scheme.
package rsmt2d

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// ExtendedDataSquare represents an extended piece of data.
type ExtendedDataSquare struct {
	*dataSquare
	codec             Codec
	originalDataWidth uint
}

func (eds *ExtendedDataSquare) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		DataSquare [][]byte `json:"data_square"`
		Codec      string   `json:"codec"`
	}{
		DataSquare: eds.dataSquare.Flattened(),
		Codec:      eds.codec.Name(),
	})
}

func (eds *ExtendedDataSquare) UnmarshalJSON(b []byte) error {
	var aux struct {
		DataSquare [][]byte `json:"data_square"`
		Codec      string   `json:"codec"`
	}

	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	codec, ok := codecs[aux.Codec]
	if !ok {
		return fmt.Errorf("unknown codec %q", aux.Codec)
	}
	importedEds, err := ImportExtendedDataSquare(aux.DataSquare, codec, NewDefaultTree)
	if err != nil {
		return err
	}
	*eds = *importedEds
	return nil
}

// ComputeExtendedDataSquare computes the extended data square for some shares
// of original data.
func ComputeExtendedDataSquare(
	data [][]byte,
	codec Codec,
	treeCreatorFn TreeConstructorFn,
) (*ExtendedDataSquare, error) {
	if len(data) > codec.MaxChunks() {
		// TODO: export this error and rename chunk to share
		return nil, errors.New("number of chunks exceeds the maximum")
	}

	shareSize := getShareSize(data)
	err := codec.ValidateChunkSize(shareSize)
	if err != nil {
		return nil, err
	}
	ds, err := newDataSquare(data, treeCreatorFn, uint(shareSize))
	if err != nil {
		return nil, err
	}

	eds := ExtendedDataSquare{dataSquare: ds, codec: codec}
	err = eds.erasureExtendSquare(codec)
	if err != nil {
		return nil, err
	}

	return &eds, nil
}

// ComputeExtendedDataSquareWithBuffer computes the extended data square for some shares
// of original data. It limits parallel operations and uses buffered NMTs to avoid allocations when computing roots.
func ComputeExtendedDataSquareWithBuffer(
	data [][]byte,
	codec Codec,
	treeCreator BufferedTreeConstructor,
) (*ExtendedDataSquare, error) {
	eds, err := ComputeExtendedDataSquare(data, codec, treeCreator.NewConstructor(uint(getWidth(data))))
	if err != nil {
		return nil, err
	}
	eds.setParallelOps(treeCreator.TreeCount())
	return eds, nil
}

// ImportExtendedDataSquare imports an extended data square, represented as flattened shares of data.
func ImportExtendedDataSquare(
	data [][]byte,
	codec Codec,
	treeCreatorFn TreeConstructorFn,
) (*ExtendedDataSquare, error) {
	if len(data) > 4*codec.MaxChunks() {
		// TODO: export this error and rename chunk to share
		return nil, errors.New("number of chunks exceeds the maximum")
	}

	shareSize := getShareSize(data)
	err := codec.ValidateChunkSize(shareSize)
	if err != nil {
		return nil, err
	}
	ds, err := newDataSquare(data, treeCreatorFn, uint(shareSize))
	if err != nil {
		return nil, err
	}

	eds := ExtendedDataSquare{dataSquare: ds, codec: codec}
	err = validateEdsWidth(eds.width)
	if err != nil {
		return nil, err
	}

	eds.originalDataWidth = eds.width / 2

	return &eds, nil
}

// NewExtendedDataSquare returns a new extended data square with a width of
// edsWidth. All shares are initialized to nil so that the returned extended
// data square can be populated via subsequent SetCell invocations.
func NewExtendedDataSquare(codec Codec, treeCreatorFn TreeConstructorFn, edsWidth uint, shareSize uint) (*ExtendedDataSquare, error) {
	err := validateEdsWidth(edsWidth)
	if err != nil {
		return nil, err
	}
	err = codec.ValidateChunkSize(int(shareSize))
	if err != nil {
		return nil, err
	}

	data := make([][]byte, edsWidth*edsWidth)
	dataSquare, err := newDataSquare(data, treeCreatorFn, shareSize)
	if err != nil {
		return nil, err
	}

	originalDataWidth := edsWidth / 2
	eds := ExtendedDataSquare{
		dataSquare:        dataSquare,
		codec:             codec,
		originalDataWidth: originalDataWidth,
	}
	return &eds, nil
}

func (eds *ExtendedDataSquare) erasureExtendSquare(codec Codec) error {
	eds.originalDataWidth = eds.width

	// Extend original square with filler shares. O represents original data. F
	// represents filler shares.
	//
	//  ------- -------
	// |       |       |
	// |   O   |   F   |
	// |       |       |
	//  ------- -------
	// |       |       |
	// |   F   |   F   |
	// |       |       |
	//  ------- -------
	if err := eds.extendSquare(eds.width, bytes.Repeat([]byte{0}, int(eds.shareSize))); err != nil {
		return err
	}

	errs, _ := errgroup.WithContext(context.Background())

	k := eds.originalDataWidth
	var arena *parityArena
	// Only allocate the arena for a complete original square, so that invalid
	// input fails in Encode as before, without the up-front allocation.
	// Exact type check: a type embedding *LeoRSCodec may override Encode.
	if enc, ok := codec.(*LeoRSCodec); ok && eds.isOriginalComplete() {
		arena = &parityArena{enc: enc, buf: make([]byte, 3*k*k*eds.shareSize), k: k, shareSize: eds.shareSize}
	}

	// Populate filler shares in Q1 and Q2. E represents erasure data.
	//
	//  ------- -------
	// |       |       |
	// |   O → |   E   |
	// |   ↓   |       |
	//  ------- -------
	// |       |       |
	// |   E   |   F   |
	// |       |       |
	//  ------- -------
	// Arena tasks: 0..k-1 are Q1 rows, k..2k-1 are Q2 columns and 2k..3k-1
	// are Q3 rows.
	for i := uint(0); i < eds.originalDataWidth; i++ {
		i := i

		// Encode Q0 and populate Q1 with erasure data
		errs.Go(func() error {
			return eds.erasureExtendRow(codec, i, arena, i)
		})

		// Encode Q0 and populate Q2 with erasure data
		errs.Go(func() error {
			return eds.erasureExtendCol(codec, i, arena, k+i)
		})
	}

	if err := errs.Wait(); err != nil {
		return err
	}

	// Populate filler shares in Q3. Note that the parity data in `Q3` will be
	// identical if it is vertically extended from `Q1` or horizontally extended
	// from `Q2`.
	//
	//  ------- -------
	// |       |       |
	// |   O   |   E   |
	// |       |       |
	//  ------- -------
	// |       |       |
	// |   E → |   E   |
	// |       |       |
	//  ------- -------
	for i := eds.originalDataWidth; i < eds.width; i++ {
		i := i

		// Encode Q2 and populate Q3 with erasure data (arena tasks 2k..3k-1)
		errs.Go(func() error {
			return eds.erasureExtendRow(codec, i, arena, k+i)
		})
	}

	return errs.Wait()
}

// parityArena hands out pre-allocated parity shares for one extension: a
// single contiguous allocation of 3*k*k shares instead of 3*k*k allocations.
type parityArena struct {
	enc       *LeoRSCodec
	buf       []byte
	k         uint
	shareSize uint
}

// slot returns the parity shares for task t (0 <= t < 3k), k shares each.
func (a *parityArena) slot(t uint) [][]byte {
	out := make([][]byte, a.k)
	base := t * a.k * a.shareSize
	for i := uint(0); i < a.k; i++ {
		off := base + i*a.shareSize
		out[i] = a.buf[off : off+a.shareSize : off+a.shareSize]
	}
	return out
}

// isOriginalComplete reports whether no share of the original square is nil.
func (eds *ExtendedDataSquare) isOriginalComplete() bool {
	for i := uint(0); i < eds.originalDataWidth; i++ {
		if !isComplete(eds.rowSlice(i, 0, eds.originalDataWidth)) {
			return false
		}
	}
	return true
}

// encode returns the parity shares of data, written into arena slot task when
// an arena is given.
func encode(codec Codec, data [][]byte, arena *parityArena, task uint) ([][]byte, error) {
	if arena == nil {
		return codec.Encode(data)
	}
	parity := arena.slot(task)
	if err := arena.enc.encodeInto(data, parity); err != nil {
		return nil, err
	}
	return parity, nil
}

func (eds *ExtendedDataSquare) erasureExtendRow(codec Codec, rowIdx uint, arena *parityArena, task uint) error {
	parityShares, err := encode(codec, eds.rowSlice(rowIdx, 0, eds.originalDataWidth), arena, task)
	if err != nil {
		return err
	}
	return eds.setRowSlice(rowIdx, eds.originalDataWidth, parityShares)
}

func (eds *ExtendedDataSquare) erasureExtendCol(codec Codec, colIdx uint, arena *parityArena, task uint) error {
	parityShares, err := encode(codec, eds.colSlice(0, colIdx, eds.originalDataWidth), arena, task)
	if err != nil {
		return err
	}
	return eds.setColSlice(colIdx, eds.originalDataWidth, parityShares)
}

func (eds *ExtendedDataSquare) deepCopy(codec Codec) (ExtendedDataSquare, error) {
	imported, err := ImportExtendedDataSquare(eds.Flattened(), codec, eds.createTreeFn)
	return *imported, err
}

// Col returns a column slice.
// This slice is a copy of the internal column slice.
func (eds *ExtendedDataSquare) Col(colIdx uint) [][]byte {
	return deepCopy(eds.col(colIdx))
}

// ColRoots returns the Merkle roots of all the columns in the square. Returns
// an error if the EDS is incomplete (i.e. some shares are nil).
func (eds *ExtendedDataSquare) ColRoots() ([][]byte, error) {
	colRoots, err := eds.getColRoots()
	if err != nil {
		return nil, err
	}
	return deepCopy(colRoots), nil
}

// Row returns a row slice.
// This slice is a copy of the internal row slice.
func (eds *ExtendedDataSquare) Row(rowIdx uint) [][]byte {
	return deepCopy(eds.row(rowIdx))
}

// RowRoots returns the Merkle roots of all the rows in the square. Returns an
// error if the EDS is incomplete (i.e. some shares are nil).
func (eds *ExtendedDataSquare) RowRoots() ([][]byte, error) {
	rowRoots, err := eds.getRowRoots()
	if err != nil {
		return nil, err
	}
	return deepCopy(rowRoots), nil
}

func deepCopy(original [][]byte) [][]byte {
	dest := make([][]byte, len(original))
	for i, cell := range original {
		if cell == nil {
			continue
		}
		dest[i] = make([]byte, len(cell))
		copy(dest[i], cell)
	}
	return dest
}

// Width returns the width of the square.
func (eds *ExtendedDataSquare) Width() uint {
	return eds.width
}

// Flattened returns the extended data square as a flattened slice of bytes.
func (eds *ExtendedDataSquare) Flattened() [][]byte {
	return deepCopy(eds.dataSquare.Flattened())
}

// FlattenedODS returns the original data square as a flattened slice of bytes.
func (eds *ExtendedDataSquare) FlattenedODS() (flattened [][]byte) {
	flattened = make([][]byte, eds.originalDataWidth*eds.originalDataWidth)
	for rowIdx := uint(0); rowIdx < eds.originalDataWidth; rowIdx++ {
		row := eds.Row(rowIdx)
		for colIdx := uint(0); colIdx < eds.originalDataWidth; colIdx++ {
			flattened[(rowIdx*eds.originalDataWidth)+colIdx] = row[colIdx]
		}
	}
	return flattened
}

// Equals returns true if other is equal to eds.
func (eds *ExtendedDataSquare) Equals(other *ExtendedDataSquare) bool {
	if eds.originalDataWidth != other.originalDataWidth {
		return false
	}
	if eds.codec.Name() != other.codec.Name() {
		return false
	}
	if eds.shareSize != other.shareSize {
		return false
	}
	if eds.width != other.width {
		return false
	}

	for rowIdx := uint(0); rowIdx < eds.Width(); rowIdx++ {
		edsRow := eds.Row(rowIdx)
		otherRow := other.Row(rowIdx)

		for colIdx := 0; colIdx < len(edsRow); colIdx++ {
			if !bytes.Equal(edsRow[colIdx], otherRow[colIdx]) {
				return false
			}
		}
	}

	return true
}

// Roots returns a byte slice with this eds's RowRoots and ColRoots
// concatenated.
func (eds *ExtendedDataSquare) Roots() (roots [][]byte, err error) {
	rowRoots, err := eds.RowRoots()
	if err != nil {
		return nil, err
	}
	colRoots, err := eds.ColRoots()
	if err != nil {
		return nil, err
	}

	roots = make([][]byte, 0, len(rowRoots)+len(colRoots))
	roots = append(roots, rowRoots...)
	roots = append(roots, colRoots...)
	return roots, nil
}

// validateEdsWidth returns an error if edsWidth is not a valid width for an
// extended data square.
func validateEdsWidth(edsWidth uint) error {
	if edsWidth == 0 {
		return fmt.Errorf("extended data square width must be greater than zero")
	}
	if edsWidth%2 != 0 {
		return fmt.Errorf("extended data square width %v must be even", edsWidth)
	}

	return nil
}

// getShareSize returns the size of the first non-nil share in data.
func getShareSize(data [][]byte) (shareSize int) {
	for _, d := range data {
		if d != nil {
			return len(d)
		}
	}
	return 0
}
