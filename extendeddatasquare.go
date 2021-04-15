// Package rsmt2d implements the two dimensional Reed-Solomon merkle tree data availability scheme.
package rsmt2d

import (
	"bytes"
	"errors"
)

// ExtendedDataSquare represents an extended piece of data.
type ExtendedDataSquare struct {
	*dataSquare
	originalDataWidth uint
}

// ComputeExtendedDataSquare computes the extended data square for some chunks of data.
func ComputeExtendedDataSquare(data [][]byte, codec Codec, treeCreatorFn TreeConstructorFn) (*ExtendedDataSquare, error) {
	if len(data) > codec.maxChunks() {
		return nil, errors.New("number of chunks exceeds the maximum")
	}

	ds, err := newDataSquare(data, treeCreatorFn)
	if err != nil {
		return nil, err
	}

	eds := ExtendedDataSquare{dataSquare: ds}
	err = eds.erasureExtendSquare(codec)
	if err != nil {
		return nil, err
	}

	return &eds, nil
}

// ImportExtendedDataSquare imports an extended data square, represented as flattened chunks of data.
func ImportExtendedDataSquare(data [][]byte, codec Codec, treeCreatorFn TreeConstructorFn) (*ExtendedDataSquare, error) {
	if len(data) > 4*codec.maxChunks() {
		return nil, errors.New("number of chunks exceeds the maximum")
	}

	ds, err := newDataSquare(data, treeCreatorFn)
	if err != nil {
		return nil, err
	}

	eds := ExtendedDataSquare{dataSquare: ds}
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

	var shares [][]byte
	var err error

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
		shares, err = codec.Encode(eds.rowSlice(i, 0, eds.originalDataWidth))
		if err != nil {
			return err
		}
		if err := eds.setRowSlice(i, eds.originalDataWidth, shares); err != nil {
			return err
		}

		// Extend vertically
		shares, err = codec.Encode(eds.columnSlice(0, i, eds.originalDataWidth))
		if err != nil {
			return err
		}
		if err := eds.setColumnSlice(eds.originalDataWidth, i, shares); err != nil {
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
		shares, err = codec.Encode(eds.rowSlice(i, 0, eds.originalDataWidth))
		if err != nil {
			return err
		}
		if err := eds.setRowSlice(i, eds.originalDataWidth, shares); err != nil {
			return err
		}
	}

	return nil
}

func (eds *ExtendedDataSquare) deepCopy(codec Codec) (ExtendedDataSquare, error) {
	eds, err := ImportExtendedDataSquare(eds.flattened(), codec, eds.createTreeFn)
	return *eds, err
}
