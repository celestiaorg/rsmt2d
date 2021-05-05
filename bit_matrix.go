package rsmt2d

import "fmt"

type bitMatrix struct {
	mask       []uint64
	squareSize int
}

func newBitMatrix(squareSize int) bitMatrix {
	bits := squareSize * squareSize
	return bitMatrix{mask: make([]uint64, (bits+63)/64), squareSize: squareSize}
}

// idx = rowIndex*squareSize+colIdx
func (bm bitMatrix) SetFlat(idx int) {
	bm.mask[idx/64] |= uint64(1) << uint(idx%64)
}

func (bm bitMatrix) Get(row, col int) bool {
	assertValidIndices(row, col, bm.squareSize)
	idx := row*bm.squareSize + col
	return bm.mask[idx/64]&(uint64(1)<<uint(idx%64)) > 0
}

func (bm *bitMatrix) Set(row, col int) {
	assertValidIndices(row, col, bm.squareSize)
	idx := row*bm.squareSize + col
	bm.mask[idx/64] |= uint64(1) << uint(idx%64)
}

func (bm bitMatrix) ColIsOne(c int) bool {
	for r := 0; r < bm.squareSize; r++ {
		if !bm.Get(r, c) {
			return false
		}
	}
	return true
}

func (bm bitMatrix) RowIsOne(r int) bool {
	for c := 0; c < bm.squareSize; c++ {
		if !bm.Get(r, c) {
			return false
		}
	}
	return true
}

func (bm bitMatrix) NumOnesInRow(r int) int {
	var counter int
	for c := 0; c < bm.squareSize; c++ {
		if bm.Get(r, c) {
			counter++
		}
	}

	return counter
}

func (bm bitMatrix) NumOnesInCol(c int) int {
	var counter int
	for r := 0; r < bm.squareSize; r++ {
		if bm.Get(r, c) {
			counter++
		}
	}

	return counter
}

func (bm bitMatrix) RowRangeIsOne(r, start, end int) bool {
	for c := start; c <= end && c < bm.squareSize; c++ {
		if !bm.Get(r, c) {
			return false
		}
	}
	return true
}

func (bm bitMatrix) ColRangeIsOne(c, start, end int) bool {
	for r := start; r <= end && r < bm.squareSize; r++ {
		if !bm.Get(r, c) {
			return false
		}
	}
	return true
}

func assertValidIndices(row, col, squareSize int) {
	if row >= squareSize || col >= squareSize {
		panic(fmt.Sprintf(
			"want: row < squareSize && col < squareSize, got: %[1]v >= %[3]v || %[2]v >= %[3]v",
			row,
			col,
			squareSize))
	}
}
