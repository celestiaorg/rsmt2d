package rsmt2d

import (
	"errors"
	"math"
)

type dataSquare struct {
	square       [][][]byte
	width        uint
	chunkSize    uint
	rowRoots     [][]byte
	columnRoots  [][]byte
	createTreeFn TreeConstructorFn
}

func newDataSquare(data [][]byte, treeCreator TreeConstructorFn) (*dataSquare, error) {
	width := int(math.Ceil(math.Sqrt(float64(len(data)))))
	if int(math.Pow(float64(width), 2)) != len(data) {
		return nil, errors.New("number of chunks must be a square number")
	}

	square := make([][][]byte, width)
	chunkSize := len(data[0])
	for i := 0; i < width; i++ {
		square[i] = data[i*width : i*width+width]

		for j := 0; j < width; j++ {
			if len(square[i][j]) != chunkSize {
				return nil, errors.New("all chunks must be of equal size")
			}
		}
	}

	return &dataSquare{
		square:       square,
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
	newSquare := make([][][]byte, newWidth)

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
		copy(row, ds.square[i])
		newSquare[i] = append(row, fillerExtendedRow...)
	}

	for i := ds.width; i < newWidth; i++ {
		newSquare[i] = make([][]byte, newWidth)
		copy(newSquare[i], fillerRow)
	}

	ds.square = newSquare
	ds.width = newWidth

	ds.resetRoots()

	return nil
}

func (ds *dataSquare) rowSlice(x uint, y uint, length uint) [][]byte {
	return ds.square[x][y : y+length]
}

// Row returns the data in a row.
func (ds *dataSquare) Row(x uint) [][]byte {
	return ds.rowSlice(x, 0, ds.width)
}

func (ds *dataSquare) setRowSlice(x uint, y uint, newRow [][]byte) error {
	for i := uint(0); i < uint(len(newRow)); i++ {
		if len(newRow[i]) != int(ds.chunkSize) {
			return errors.New("invalid chunk size")
		}
	}

	for i := uint(0); i < uint(len(newRow)); i++ {
		ds.square[x][y+i] = newRow[i]
	}

	ds.resetRoots()

	return nil
}

func (ds *dataSquare) columnSlice(x uint, y uint, length uint) [][]byte {
	columnSlice := make([][]byte, length)
	for i := uint(0); i < length; i++ {
		columnSlice[i] = ds.square[x+i][y]
	}

	return columnSlice
}

// Column returns the data in a column.
func (ds *dataSquare) Column(y uint) [][]byte {
	return ds.columnSlice(0, y, ds.width)
}

func (ds *dataSquare) setColumnSlice(x uint, y uint, newColumn [][]byte) error {
	for i := uint(0); i < uint(len(newColumn)); i++ {
		if len(newColumn[i]) != int(ds.chunkSize) {
			return errors.New("invalid chunk size")
		}
	}

	for i := uint(0); i < uint(len(newColumn)); i++ {
		ds.square[x+i][y] = newColumn[i]
	}

	ds.resetRoots()

	return nil
}

func (ds *dataSquare) resetRoots() {
	ds.rowRoots = nil
	ds.columnRoots = nil
}

func (ds *dataSquare) computeRoots() {
	rowRoots := make([][]byte, ds.width)
	columnRoots := make([][]byte, ds.width)
	for i := uint(0); i < ds.width; i++ {
		rowRoots[i] = ds.RowRoot(i)
		columnRoots[i] = ds.ColRoot(i)
	}

	ds.rowRoots = rowRoots
	ds.columnRoots = columnRoots
}

// RowRoots returns the Merkle roots of all the rows in the square.
func (ds *dataSquare) RowRoots() [][]byte {
	if ds.rowRoots == nil {
		ds.computeRoots()
	}

	return ds.rowRoots
}

// RowRoot calculates and returns the root of the selected row. Note: unlike the
// RowRoots method, RowRoot uses the built-in cache when available.
func (ds *dataSquare) RowRoot(x uint) []byte {
	if ds.rowRoots != nil {
		return ds.rowRoots[x]
	}

	tree := ds.createTreeFn()
	for i, d := range ds.Row(x) {
		tree.Push(d, SquareIndex{Cell: uint(i), Axis: x})
	}

	return tree.Root()
}

// ColumnRoots returns the Merkle roots of all the columns in the square.
func (ds *dataSquare) ColumnRoots() [][]byte {
	if ds.columnRoots == nil {
		ds.computeRoots()
	}

	return ds.columnRoots
}

// ColRoot calculates and returns the root of the selected row. Note: unlike the
// ColRoots method, ColRoot does not use the built in cache
func (ds *dataSquare) ColRoot(y uint) []byte {
	if ds.columnRoots != nil {
		return ds.columnRoots[y]
	}

	tree := ds.createTreeFn()
	for i, d := range ds.Column(y) {
		tree.Push(d, SquareIndex{Axis: y, Cell: uint(i)})
	}

	return tree.Root()
}

func (ds *dataSquare) computeRowProof(x uint, y uint) ([]byte, [][]byte, uint, uint, error) {
	tree := ds.createTreeFn()
	data := ds.Row(x)

	for i := uint(0); i < ds.width; i++ {
		tree.Push(data[i], SquareIndex{Axis: y, Cell: uint(i)})
	}

	merkleRoot, proof, proofIndex, numLeaves := tree.Prove(int(y))
	return merkleRoot, proof, uint(proofIndex), uint(numLeaves), nil
}

func (ds *dataSquare) computeColumnProof(x uint, y uint) ([]byte, [][]byte, uint, uint, error) {
	tree := ds.createTreeFn()
	data := ds.Column(y)

	for i := uint(0); i < ds.width; i++ {
		tree.Push(data[i], SquareIndex{Axis: y, Cell: uint(i)})
	}
	// TODO(ismail): check for overflow when casting from uint -> int
	merkleRoot, proof, proofIndex, numLeaves := tree.Prove(int(x))
	return merkleRoot, proof, uint(proofIndex), uint(numLeaves), nil
}

// Cell returns a single chunk at a specific cell.
func (ds *dataSquare) Cell(x uint, y uint) []byte {
	cell := make([]byte, ds.chunkSize)
	copy(cell, ds.square[x][y])
	return cell
}

func (ds *dataSquare) setCell(x uint, y uint, newChunk []byte) {
	ds.square[x][y] = newChunk
	ds.resetRoots()
}

func (ds *dataSquare) flattened() [][]byte {
	flattened := [][]byte(nil)
	for _, data := range ds.square {
		flattened = append(flattened, data...)
	}

	return flattened
}

// Width returns the width of the square.
func (ds *dataSquare) Width() uint {
	return ds.width
}
