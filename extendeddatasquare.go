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
	codec             Codec
}

// ComputeExtendedDataSquare computes the extended data square for some chunks of data.
func ComputeExtendedDataSquare(data [][]byte, codec Codec) (*ExtendedDataSquare, error) {
	if value, ok := CodecsMaxChunksMap[codec]; ok {
		if len(data) > value {
			return nil, errors.New("number of chunks exceeds the maximum")
		}
	} else {
		return nil, errors.New("unsupported codec")
	}

	ds, err := newDataSquare(data)
	if err != nil {
		return nil, err
	}

	eds := ExtendedDataSquare{dataSquare: ds, codec: codec}
	err = eds.erasureExtendSquare()
	if err != nil {
		return nil, err
	}

	return &eds, nil
}

// ImportExtendedDataSquare imports an extended data square, represented as flattened chunks of data.
func ImportExtendedDataSquare(data [][]byte, codec Codec) (*ExtendedDataSquare, error) {
	if value, ok := CodecsMaxChunksMap[codec]; ok {
		if len(data) > value*4 {
			return nil, errors.New("number of chunks exceeds the maximum")
		}
	} else {
		return nil, errors.New("unsupported codec")
	}

	ds, err := newDataSquare(data)
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

func (eds *ExtendedDataSquare) erasureExtendSquare() error {
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
		shares, err = encode(eds.rowSlice(i, 0, eds.originalDataWidth), eds.codec)
		if err != nil {
			return err
		}
		if err := eds.setRowSlice(i, eds.originalDataWidth, shares); err != nil {
			return err
		}

		// Extend vertically
		shares, err = encode(eds.columnSlice(0, i, eds.originalDataWidth), eds.codec)
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
		shares, err = encode(eds.rowSlice(i, 0, eds.originalDataWidth), eds.codec)
		if err != nil {
			return err
		}
		if err := eds.setRowSlice(i, eds.originalDataWidth, shares); err != nil {
			return err
		}
	}

	return nil
}

func (eds *ExtendedDataSquare) deepCopy() (ExtendedDataSquare, error) {
	eds, err := ImportExtendedDataSquare(eds.flattened(), eds.codec)
	return *eds, err
}
