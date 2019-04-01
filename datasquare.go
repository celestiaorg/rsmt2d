package rsmt2d

import(
    "hash"
    "math"
    "errors"
    "crypto/sha256"

    "github.com/NebulousLabs/merkletree"
)

type dataSquare struct {
    square [][][]byte
    width uint
    chunkSize uint
    rowRoots [][]byte
    columnRoots [][]byte
    hasher hash.Hash
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

    return &dataSquare{
        square: square,
        width: uint(width),
        chunkSize: uint(chunkSize),
        hasher: sha256.New(),
    }, nil
}

// SetHasher sets the hasher used for computing Merkle roots.
func (ds *dataSquare) SetHasher(hasher hash.Hash) {
    ds.hasher = hasher
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
    return ds.square[x][y:y+length]
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
    var rowTree *merkletree.Tree
    var columnTree *merkletree.Tree
    var rowData [][]byte
    var columnData [][]byte
    for i := uint(0); i < ds.width; i++ {
        rowTree = merkletree.New(ds.hasher)
        columnTree = merkletree.New(ds.hasher)
        rowData = ds.Row(i)
        columnData = ds.Column(i)
        for j := uint(0); j < ds.width; j++ {
            rowTree.Push(rowData[j])
            columnTree.Push(columnData[j])
        }

        rowRoots[i] = rowTree.Root()
        columnRoots[i] = columnTree.Root()
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

// ColumnRoots returns the Merkle roots of all the columns in the square.
func (ds *dataSquare) ColumnRoots() [][]byte {
    if ds.columnRoots == nil {
        ds.computeRoots()
    }

    return ds.columnRoots
}

func (ds *dataSquare) computeRowProof(x uint, y uint) ([]byte, [][]byte, uint, uint) {
    tree := merkletree.New(ds.hasher)
    tree.SetIndex(uint64(y))
    data := ds.Row(x)

    for i := uint(0); i < ds.width; i++ {
        tree.Push(data[i])
    }

    merkleRoot, proof, proofIndex, numLeaves := tree.Prove()
    return merkleRoot, proof, uint(proofIndex), uint(numLeaves)
}

func (ds *dataSquare) computeColumnProof(x uint, y uint) ([]byte, [][]byte, uint, uint) {
    tree := merkletree.New(ds.hasher)
    tree.SetIndex(uint64(x))
    data := ds.Column(y)

    for i := uint(0); i < ds.width; i++ {
        tree.Push(data[i])
    }

    merkleRoot, proof, proofIndex, numLeaves := tree.Prove()
    return merkleRoot, proof, uint(proofIndex), uint(numLeaves)
}

func (ds *dataSquare) cell(x uint, y uint) []byte {
    return ds.square[x][y]
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
