// A two dimensional Reed-Solomon merkle tree data availability scheme.
package rsmt2d

import (
    "bytes"
    "errors"

    "github.com/vivint/infectious"
)

// The max number of original data chunks
const MaxChunks = 16384 // Using Galois Field 256 correcting up to t/2 symbols

// Represents an extended piece of data.
type ExtendedDataSquare struct {
    *dataSquare
    originalDataWidth uint
}

// Loads original data as extended data.
func NewExtendedDataSquare(data [][]byte) (*ExtendedDataSquare, error) {
    if len(data) > MaxChunks {
        return nil, errors.New("number of chunks exceeds the maximum")
    }

    ds, err := newDataSquare(data)
    if err != nil {
        return nil, err
    }

    eds := ExtendedDataSquare{dataSquare: ds}
    err = eds.erasureExtendSquare()
    if (err != nil) {
        return nil, err
    }

    return &eds, nil
}

func (eds *ExtendedDataSquare) erasureExtendSquare() error {
    eds.originalDataWidth = eds.width
    eds.extendSquare(eds.width, bytes.Repeat([]byte{0}, int(eds.chunkSize)))

    fec, err := infectious.NewFEC(int(eds.originalDataWidth), int(eds.width))
    if err != nil {
        return err
    }

    shares := make([][]byte, eds.originalDataWidth)
    output := func(s infectious.Share) {
        if s.Number >= int(eds.originalDataWidth) {
            shareData := make([]byte, eds.chunkSize)
            copy(shareData, s.Data)
            shares[s.Number-int(eds.originalDataWidth)] = shareData
        }
    }

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
        err = fec.Encode(flattenChunks(eds.getRowSlice(i, 0, eds.originalDataWidth)), output)
        if err != nil {
            return err
        }
        eds.setRowSlice(i, eds.originalDataWidth, shares)

        // Extend vertically
        err = fec.Encode(flattenChunks(eds.getColumnSlice(0, i, eds.originalDataWidth)), output)
        if err != nil {
            return err
        }
        eds.setColumnSlice(eds.originalDataWidth, i, shares)
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
        err = fec.Encode(flattenChunks(eds.getRowSlice(i, 0, eds.originalDataWidth)), output)
        if err != nil {
            return err
        }
        eds.setRowSlice(i, eds.originalDataWidth, shares)
    }

    return nil
}
