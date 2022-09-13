// Package rsmt2d implements the two dimensional Reed-Solomon merkle tree data availability scheme.
package rsmt2d

import (
	"bytes"
	"context"
	"errors"

	"golang.org/x/sync/errgroup"
)

// ExtendedDataSquare represents an extended piece of data.
type ExtendedDataSquare struct {
	*dataSquare
	codec             Codec
	originalDataWidth uint
}

// ComputeExtendedDataSquare computes the extended data square for some chunks of data.
func ComputeExtendedDataSquare(
	data [][]byte,
	codec Codec,
	treeCreatorFn TreeConstructorFn,
) (*ExtendedDataSquare, error) {
	if len(data) > codec.maxChunks() {
		return nil, errors.New("number of chunks exceeds the maximum")
	}

	ds, err := newDataSquare(data, treeCreatorFn)
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

// ImportExtendedDataSquare imports an extended data square, represented as flattened chunks of data.
func ImportExtendedDataSquare(
	data [][]byte,
	codec Codec,
	treeCreatorFn TreeConstructorFn,
) (*ExtendedDataSquare, error) {
	if len(data) > 4*codec.maxChunks() {
		return nil, errors.New("number of chunks exceeds the maximum")
	}

	ds, err := newDataSquare(data, treeCreatorFn)
	if err != nil {
		return nil, err
	}

	eds := ExtendedDataSquare{dataSquare: ds, codec: codec}
	if eds.width%2 != 0 {
		return nil, errors.New("square width must be even")
	}

	eds.originalDataWidth = eds.width / 2

	return &eds, nil
}

func (eds *ExtendedDataSquare) erasureExtendSquare(codec Codec) error {
	eds.originalDataWidth = eds.width
	if err := eds.extendSquare(eds.width, bytes.Repeat([]byte{0}, int(eds.chunkSize))); err != nil {
		return err
	}

	errs, _ := errgroup.WithContext(context.Background())

	// Extend original square horizontally and vertically
	//  ------- -------
	// |       |       |
	// |   O → |   E   |
	// |   ↓   |       |
	//  ------- -------
	// |       |
	// |   E   |
	// |       |
	//  -------
	for i := uint(0); i < eds.originalDataWidth; i++ {
		// Extend horizontally
		errs.Go(func() error {
			return eds.erasureExtendRow(codec, i)
		})

		// Extend vertically
		errs.Go(func() error {
			return eds.erasureExtendCol(codec, i)
		})

		if err := errs.Wait(); err != nil {
			return err
		}
	}

	// Extend extended square horizontally
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
		// Extend horizontally
		errs.Go(func() error {
			return eds.erasureExtendRow(codec, i)
		})

		if err := errs.Wait(); err != nil {
			return err
		}
	}

	return nil
}

func (eds *ExtendedDataSquare) erasureExtendRow(codec Codec, i uint) error {
	var shares [][]byte
	var err error

	shares, err = codec.Encode(eds.rowSlice(i, 0, eds.originalDataWidth))
	if err != nil {
		return err
	}
	if err := eds.setRowSlice(i, eds.originalDataWidth, shares[len(shares)-int(eds.originalDataWidth):]); err != nil {
		return err
	}
	return nil
}

func (eds *ExtendedDataSquare) erasureExtendCol(codec Codec, i uint) error {
	var shares [][]byte
	var err error

	shares, err = codec.Encode(eds.colSlice(0, i, eds.originalDataWidth))
	if err != nil {
		return err
	}
	if err := eds.setColSlice(eds.originalDataWidth, i, shares[len(shares)-int(eds.originalDataWidth):]); err != nil {
		return err
	}
	return nil
}

func (eds *ExtendedDataSquare) deepCopy(codec Codec) (ExtendedDataSquare, error) {
	eds, err := ImportExtendedDataSquare(eds.Flattened(), codec, eds.createTreeFn)
	return *eds, err
}

// Col returns a column slice.
// This slice is a copy of the internal column slice.
func (eds *ExtendedDataSquare) Col(y uint) [][]byte {
	col := make([][]byte, eds.width)
	original := eds.col(y)
	for i, cell := range original {
		col[i] = make([]byte, eds.chunkSize)
		copy(col[i], cell)
	}
	return col
}

// ColRoots returns the Merkle roots of all the columns in the square.
func (eds *ExtendedDataSquare) ColRoots() [][]byte {
	return deepCopy(eds.getColRoots())
}

// Row returns a row slice.
// This slice is a copy of the internal row slice.
func (eds *ExtendedDataSquare) Row(x uint) [][]byte {
	return deepCopy(eds.row(x))
}

// RowRoots returns the Merkle roots of all the rows in the square.
func (eds *ExtendedDataSquare) RowRoots() [][]byte {
	return deepCopy(eds.getRowRoots())
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
