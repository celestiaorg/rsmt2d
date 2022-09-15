package rsmt2d

import (
	"errors"
	"fmt"
	"math"
	"sync"
)

// ErrUnevenChunks is thrown when non-nil chunks are not all of equal size.
var ErrUnevenChunks = errors.New("non-nil chunks not all of equal size")

// dataSquare stores all data for an original data square (ODS) or extended
// data square (EDS). Data is duplicated in both row-major and column-major
// order in order to be able to provide zero-allocation column slices.
type dataSquare struct {
	squareRow    [][][]byte // row-major
	squareCol    [][][]byte // col-major
	dataMutex    sync.Mutex
	width        uint
	chunkSize    uint
	rowRoots     [][]byte
	colRoots     [][]byte
	createTreeFn TreeConstructorFn
}

func newDataSquare(data [][]byte, treeCreator TreeConstructorFn) (*dataSquare, error) {
	width := int(math.Ceil(math.Sqrt(float64(len(data)))))
	if width*width != len(data) {
		return nil, errors.New("number of chunks must be a square number")
	}

	var chunkSize int
	for _, d := range data {
		if d != nil {
			if chunkSize == 0 {
				chunkSize = len(d)
			} else if chunkSize != len(d) {
				return nil, ErrUnevenChunks
			}
		}
	}

	squareRow := make([][][]byte, width)
	for i := 0; i < width; i++ {
		squareRow[i] = data[i*width : i*width+width]

		for j := 0; j < width; j++ {
			if squareRow[i][j] != nil && len(squareRow[i][j]) != chunkSize {
				return nil, errors.New("all chunks must be of equal size")
			}
		}
	}

	squareCol := make([][][]byte, width)
	for j := 0; j < width; j++ {
		squareCol[j] = make([][]byte, width)
		for i := 0; i < width; i++ {
			squareCol[j][i] = data[i*width+j]
		}
	}

	return &dataSquare{
		squareRow:    squareRow,
		squareCol:    squareCol,
		width:        uint(width),
		chunkSize:    uint(chunkSize),
		createTreeFn: treeCreator,
	}, nil
}

func (ds *dataSquare) extendSquare(extendedWidth uint, fillerChunk []byte) error {
	if uint(len(fillerChunk)) != ds.chunkSize {
		return errors.New("filler chunk size does not match data square chunk size")
	}

	newWidth := ds.width + extendedWidth
	newSquareRow := make([][][]byte, newWidth)

	fillerExtendedRow := make([][]byte, extendedWidth)
	for i := uint(0); i < extendedWidth; i++ {
		fillerExtendedRow[i] = fillerChunk
	}

	fillerRow := make([][]byte, newWidth)
	for i := uint(0); i < newWidth; i++ {
		fillerRow[i] = fillerChunk
	}

	row := make([][]byte, ds.width)
	for i := uint(0); i < ds.width; i++ {
		copy(row, ds.squareRow[i])
		newSquareRow[i] = append(row, fillerExtendedRow...)
	}

	for i := ds.width; i < newWidth; i++ {
		newSquareRow[i] = make([][]byte, newWidth)
		copy(newSquareRow[i], fillerRow)
	}

	ds.squareRow = newSquareRow

	newSquareCol := make([][][]byte, newWidth)
	for j := uint(0); j < newWidth; j++ {
		newSquareCol[j] = make([][]byte, newWidth)
		for i := uint(0); i < newWidth; i++ {
			newSquareCol[j][i] = newSquareRow[i][j]
		}
	}
	ds.squareCol = newSquareCol
	ds.width = newWidth

	ds.resetRoots()

	return nil
}

func (ds *dataSquare) rowSlice(x uint, y uint, length uint) [][]byte {
	return ds.squareRow[x][y : y+length]
}

// row returns a row slice.
// Do not modify this slice directly, instead use SetCell.
func (ds *dataSquare) row(x uint) [][]byte {
	return ds.rowSlice(x, 0, ds.width)
}

func (ds *dataSquare) setRowSlice(x uint, y uint, newRow [][]byte) error {
	for i := uint(0); i < uint(len(newRow)); i++ {
		if len(newRow[i]) != int(ds.chunkSize) {
			return errors.New("invalid chunk size")
		}
	}

	ds.dataMutex.Lock()
	defer ds.dataMutex.Unlock()

	for i := uint(0); i < uint(len(newRow)); i++ {
		ds.squareRow[x][y+i] = newRow[i]
		ds.squareCol[y+i][x] = newRow[i]
	}

	ds.resetRoots()

	return nil
}

func (ds *dataSquare) colSlice(x uint, y uint, length uint) [][]byte {
	return ds.squareCol[y][x : x+length]
}

// col returns a column slice.
// Do not modify this slice directly, instead use SetCell.
func (ds *dataSquare) col(y uint) [][]byte {
	return ds.colSlice(0, y, ds.width)
}

func (ds *dataSquare) setColSlice(x uint, y uint, newCol [][]byte) error {
	for i := uint(0); i < uint(len(newCol)); i++ {
		if len(newCol[i]) != int(ds.chunkSize) {
			return errors.New("invalid chunk size")
		}
	}

	ds.dataMutex.Lock()
	defer ds.dataMutex.Unlock()

	for i := uint(0); i < uint(len(newCol)); i++ {
		ds.squareRow[x+i][y] = newCol[i]
		ds.squareCol[y][x+i] = newCol[i]
	}

	ds.resetRoots()

	return nil
}

func (ds *dataSquare) resetRoots() {
	ds.rowRoots = nil
	ds.colRoots = nil
}

func (ds *dataSquare) computeRoots() {
	var wg sync.WaitGroup

	rowRoots := make([][]byte, ds.width)
	colRoots := make([][]byte, ds.width)

	for i := uint(0); i < ds.width; i++ {
		wg.Add(2)

		go func(i uint) {
			defer wg.Done()
			rowRoots[i] = ds.getRowRoot(i)
		}(i)

		go func(i uint) {
			defer wg.Done()
			colRoots[i] = ds.getColRoot(i)
		}(i)
	}

	wg.Wait()
	ds.rowRoots = rowRoots
	ds.colRoots = colRoots
}

// getRowRoots returns the Merkle roots of all the rows in the square.
func (ds *dataSquare) getRowRoots() [][]byte {
	if ds.rowRoots == nil {
		ds.computeRoots()
	}

	return ds.rowRoots
}

// getRowRoot calculates and returns the root of the selected row. Note: unlike the
// getRowRoots method, getRowRoot uses the built-in cache when available.
func (ds *dataSquare) getRowRoot(x uint) []byte {
	if ds.rowRoots != nil {
		return ds.rowRoots[x]
	}

	tree := ds.createTreeFn()
	for i, d := range ds.row(x) {
		tree.Push(d, SquareIndex{Cell: uint(i), Axis: x})
	}

	return tree.Root()
}

// getColRoots returns the Merkle roots of all the columns in the square.
func (ds *dataSquare) getColRoots() [][]byte {
	if ds.colRoots == nil {
		ds.computeRoots()
	}

	return ds.colRoots
}

// getColRoot calculates and returns the root of the selected row. Note: unlike the
// getColRoots method, getColRoot uses the built-in cache when available.
func (ds *dataSquare) getColRoot(y uint) []byte {
	if ds.colRoots != nil {
		return ds.colRoots[y]
	}

	tree := ds.createTreeFn()
	for i, d := range ds.col(y) {
		tree.Push(d, SquareIndex{Axis: y, Cell: uint(i)})
	}

	return tree.Root()
}

// GetCell returns a copy of a specific cell.
func (ds *dataSquare) GetCell(x uint, y uint) []byte {
	if ds.squareRow[x][y] == nil {
		return nil
	}
	cell := make([]byte, ds.chunkSize)
	copy(cell, ds.squareRow[x][y])
	return cell
}

// SetCell sets a specific cell. Cell to set must be `nil`.
// Panics if attempting to set a cell that is not `nil`.
func (ds *dataSquare) SetCell(x uint, y uint, newChunk []byte) {
	if ds.squareRow[x][y] != nil {
		panic(fmt.Sprintf("cannot set cell (%d, %d) as it already has a value %x", x, y, ds.squareRow[x][y]))
	}
	ds.squareRow[x][y] = newChunk
	ds.squareCol[y][x] = newChunk
	ds.resetRoots()
}

// setCell sets a specific cell.
func (ds *dataSquare) setCell(x uint, y uint, newChunk []byte) {
	ds.squareRow[x][y] = newChunk
	ds.squareCol[y][x] = newChunk
	ds.resetRoots()
}

// Flattened returns the concatenated rows of the data square.
func (ds *dataSquare) Flattened() [][]byte {
	flattened := [][]byte(nil)
	for _, data := range ds.squareRow {
		flattened = append(flattened, data...)
	}

	return flattened
}
