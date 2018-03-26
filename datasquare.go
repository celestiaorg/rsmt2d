package rsmt2d

import(
    "math"
    "errors"
)

type dataSquare struct {
    square [][][]byte
    width uint
    chunkSize uint
}

func newDataSquare(data [][]byte) (*dataSquare, error) {
    width := int(math.Ceil(math.Sqrt(float64(len(data)))))
    if int(math.Pow(float64(width), 2)) != len(data) {
        return nil, errors.New("number of chunks must be a power of 2")
    }

    square := make([][][]byte, width)
    chunkSize := len(data[0])
    for i := 0; i < width; i++ {
        square[i] = data[i*width:i*width+width]

        for j := 0; j < width; j++ {
            if len(square[i][j]) != chunkSize {
                return nil, errors.New("all chunks must be of equal size")
            }
        }
    }

    return &dataSquare{square, uint(width), uint(chunkSize)}, nil
}

func (ds *dataSquare) extendSquare(extendedWidth uint, fillerChunk []byte) error {
    if (uint(len(fillerChunk)) != ds.chunkSize) {
        return errors.New("filler chunk size does not match data square chunk size")
    }

    newWidth := ds.width + extendedWidth
    newSquare := make([][][]byte, newWidth)

    fillerExtendedRow := make([][]byte, extendedWidth)
    for i := uint(0); i < extendedWidth; i++ {
        fillerExtendedRow[i] = fillerChunk
    }

    fillerRow := make([][]byte, newWidth)
    for i := uint(0); i < newWidth; i++ {
        fillerRow[i] = fillerChunk
    }

    for i := uint(0); i < ds.width; i++ {
        newSquare[i] = append(ds.square[i], fillerExtendedRow...)
    }

    for i := ds.width; i < newWidth; i++ {
        newSquare[i] = fillerRow
    }

    ds.square = newSquare

    return nil
}
