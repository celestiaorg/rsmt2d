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
	treeName          string
	originalDataWidth uint
}

func (eds *ExtendedDataSquare) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		DataSquare [][]byte `json:"data_square"`
		Codec      string   `json:"codec"`
		Tree       string   `json:"tree"`
	}{
		DataSquare: eds.dataSquare.Flattened(),
		Codec:      eds.codec.Name(),
		Tree:       eds.treeName,
	})
}

func (eds *ExtendedDataSquare) UnmarshalJSON(b []byte) error {
	var aux struct {
		DataSquare [][]byte `json:"data_square"`
		Codec      string   `json:"codec"`
		Tree       string   `json:"tree"`
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return err
	}

	if aux.Tree == "" {
		aux.Tree = DefaultTreeName
	}

	importedEds, err := ImportExtendedDataSquare(aux.DataSquare, codecs[aux.Codec], aux.Tree)
	if err != nil {
		return err
	}
	*eds = *importedEds
	return nil
}

// ComputeExtendedDataSquare computes the extended data square for some chunks
// of original data.
func ComputeExtendedDataSquare(
	data [][]byte,
	codec Codec,
	treeName string,
) (*ExtendedDataSquare, error) {
	if len(data) > codec.MaxChunks() {
		return nil, errors.New("number of chunks exceeds the maximum")
	}

	chunkSize := getChunkSize(data)
	err := codec.ValidateChunkSize(chunkSize)
	if err != nil {
		return nil, err
	}

	treeCreatorFn, err := TreeFn(treeName)
	if err != nil {
		return nil, err
	}

	ds, err := newDataSquare(data, treeCreatorFn, uint(chunkSize))
	if err != nil {
		return nil, err
	}

	eds := ExtendedDataSquare{dataSquare: ds, codec: codec, treeName: treeName}
	err = eds.erasureExtendSquare(codec)
	if err != nil {
		return nil, err
	}

	return &eds, nil
}

// ImportExtendedDataSquare imports an extended data square, represented as flattened chunks of data.
func ImportExtendedDataSquare(
	data [][]byte,
	codec Codec,
	treeName string,
) (*ExtendedDataSquare, error) {
	if len(data) > 4*codec.MaxChunks() {
		return nil, errors.New("number of chunks exceeds the maximum")
	}

	chunkSize := getChunkSize(data)
	err := codec.ValidateChunkSize(chunkSize)
	if err != nil {
		return nil, err
	}

	treeCreatorFn, err := TreeFn(treeName)
	if err != nil {
		return nil, err
	}

	ds, err := newDataSquare(data, treeCreatorFn, uint(chunkSize))
	if err != nil {
		return nil, err
	}

	eds := ExtendedDataSquare{dataSquare: ds, codec: codec, treeName: treeName}
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
func NewExtendedDataSquare(codec Codec, treeCreatorFn TreeConstructorFn, edsWidth uint, chunkSize uint) (*ExtendedDataSquare, error) {
	err := validateEdsWidth(edsWidth)
	if err != nil {
		return nil, err
	}
	err = codec.ValidateChunkSize(int(chunkSize))
	if err != nil {
		return nil, err
	}

	data := make([][]byte, edsWidth*edsWidth)
	dataSquare, err := newDataSquare(data, treeCreatorFn, chunkSize)
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

	// Extend original square with filler chunks. O represents original data. F
	// represents filler chunks.
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
	if err := eds.extendSquare(eds.width, bytes.Repeat([]byte{0}, int(eds.chunkSize))); err != nil {
		return err
	}

	errs, _ := errgroup.WithContext(context.Background())

	// Populate filler chunks in Q1 and Q2. E represents erasure data.
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
	for i := uint(0); i < eds.originalDataWidth; i++ {
		i := i

		// Encode Q0 and populate Q1 with erasure data
		errs.Go(func() error {
			return eds.erasureExtendRow(codec, i)
		})

		// Encode Q0 and populate Q2 with erasure data
		errs.Go(func() error {
			return eds.erasureExtendCol(codec, i)
		})
	}

	if err := errs.Wait(); err != nil {
		return err
	}

	// Populate filler chunks in Q3. Note that the parity data in `Q3` will be
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

		// Encode Q2 and populate Q3 with erasure data
		errs.Go(func() error {
			return eds.erasureExtendRow(codec, i)
		})
	}

	return errs.Wait()
}

func (eds *ExtendedDataSquare) erasureExtendRow(codec Codec, i uint) error {
	parityShares, err := codec.Encode(eds.rowSlice(i, 0, eds.originalDataWidth))
	if err != nil {
		return err
	}
	return eds.setRowSlice(i, eds.originalDataWidth, parityShares)
}

func (eds *ExtendedDataSquare) erasureExtendCol(codec Codec, i uint) error {
	parityShares, err := codec.Encode(eds.colSlice(0, i, eds.originalDataWidth))
	if err != nil {
		return err
	}
	return eds.setColSlice(eds.originalDataWidth, i, parityShares)
}

func (eds *ExtendedDataSquare) deepCopy(codec Codec) (ExtendedDataSquare, error) {
	imported, err := ImportExtendedDataSquare(eds.Flattened(), codec, eds.treeName)
	return *imported, err
}

// Col returns a column slice.
// This slice is a copy of the internal column slice.
func (eds *ExtendedDataSquare) Col(y uint) [][]byte {
	return deepCopy(eds.col(y))
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
func (eds *ExtendedDataSquare) Row(x uint) [][]byte {
	return deepCopy(eds.row(x))
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
	return eds.dataSquare.Flattened()
}

// FlattenedODS returns the original data square as a flattened slice of bytes.
func (eds *ExtendedDataSquare) FlattenedODS() (flattened [][]byte) {
	flattened = make([][]byte, eds.originalDataWidth*eds.originalDataWidth)
	for i := uint(0); i < eds.originalDataWidth; i++ {
		row := eds.Row(i)
		for j := uint(0); j < eds.originalDataWidth; j++ {
			flattened[(i*eds.originalDataWidth)+j] = row[j]
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
	if eds.chunkSize != other.chunkSize {
		return false
	}
	if eds.width != other.width {
		return false
	}

	for rowIndex := uint(0); rowIndex < eds.Width(); rowIndex++ {
		edsRow := eds.Row(rowIndex)
		otherRow := other.Row(rowIndex)

		for colIndex := 0; colIndex < len(edsRow); colIndex++ {
			if !bytes.Equal(edsRow[colIndex], otherRow[colIndex]) {
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
	if edsWidth%2 != 0 {
		return fmt.Errorf("extended data square width %v must be even", edsWidth)
	}

	return nil
}

// getChunkSize returns the size of the first non-nil chunk in data.
func getChunkSize(data [][]byte) (chunkSize int) {
	for _, d := range data {
		if d != nil {
			return len(d)
		}
	}
	return 0
}
