package rsmt2d

import (
    "bytes"
    "fmt"
    "errors"
    "reflect"

    "gonum.org/v1/gonum/mat"
    "github.com/vivint/infectious"
)

const (
    row = 0
    column = 1
)

// ByzantineRowError is thrown when there is a repaired row does not match the expected row merkle root.
type ByzantineRowError struct {
    RowNumber uint
    LastGoodSquare ExtendedDataSquare
}

func (e *ByzantineRowError) Error() string {
    return fmt.Sprintf("byzantine row: %d", e.RowNumber)
}

// ByzantineColumnError is thrown when there is a repaired column does not match the expected column merkle root.
type ByzantineColumnError struct {
    ColumnNumber uint
    LastGoodSquare ExtendedDataSquare
}

func (e *ByzantineColumnError) Error() string {
    return fmt.Sprintf("byzantine column: %d", e.ColumnNumber)
}

// UnrepairableDataSquareError is thrown when there is insufficient chunks to repair the square.
type UnrepairableDataSquareError struct {
}

func (e *UnrepairableDataSquareError) Error() string {
    return "failed to solve data square"
}

// RepairExtendedDataSquare repairs an incomplete extended data square, against its expected row and column merkle roots.
// Missing data chunks should be represented as nil.
func RepairExtendedDataSquare(rowRoots [][]byte, columnRoots [][]byte, data [][]byte) (*ExtendedDataSquare, error) {
    matrixData := make([]float64, len(data))
    var chunkSize int
    for i := range data {
        if data[i] == nil {
            matrixData[i] = 0
        } else {
            matrixData[i] = 1
            if chunkSize == 0 {
                chunkSize = len(data[i])
            }
        }
    }

    if chunkSize == 0 {
        return nil, &UnrepairableDataSquareError{}
    }

    fillerChunk := bytes.Repeat([]byte{0}, chunkSize)
    for i := range data {
        if data[i] == nil {
            data[i] = make([]byte, chunkSize)
            copy(data[i], fillerChunk)
        }
    }

    eds, err := ImportExtendedDataSquare(data)
    if err != nil {
        return nil, err
    }

    matrix := mat.NewDense(int(eds.width), int(eds.width), matrixData)

    err = eds.prerepairSanityCheck(rowRoots, columnRoots, *matrix)
    if err != nil {
        return nil, err
    }

    err = eds.solveCrossword(rowRoots, columnRoots, *matrix)
    if err != nil {
        return nil, err
    }

    return eds, err
}

func (eds *ExtendedDataSquare) solveCrossword(rowRoots [][]byte, columnRoots [][]byte, mask mat.Dense) error {
    rebuiltShares := make([]infectious.Share, eds.originalDataWidth)
    rebuiltSharesOutput := func(s infectious.Share) {
        rebuiltShares[s.Number] = s.DeepCopy()
    }
    rebuiltExtendedSharesOutput := func(s infectious.Share) {
        if s.Number >= int(eds.originalDataWidth) {
            rebuiltShares[s.Number-int(eds.originalDataWidth)] = s.DeepCopy()
        }
    }

    fec, err := infectious.NewFEC(int(eds.originalDataWidth), int(eds.width))
    if err != nil {
        return err
    }

    // Keep repeating until the square is solved
    var solved bool
    var progressMade bool
    for {
        solved = true
        progressMade = false

        // Loop through every row and column, attempt to rebuild each row or column if incomplete
        for i := uint(0); i < eds.width; i++ {
            for mode := range []int{row, column} {
                var vectorMask mat.Vector
                if mode == row {
                    vectorMask = mask.RowView(int(i))
                } else if mode == column {
                    vectorMask = mask.ColView(int(i))
                }

                if !vecIsTrue(vectorMask) { // row/column incomplete
                    // Prepare shares
                    var vectorData [][]byte
                    if mode == row {
                        vectorData = eds.row(i)
                    } else if mode == column {
                        vectorData = eds.column(i)
                    }
                    shares := []infectious.Share{}
                    for j := 0; j < vectorMask.Len(); j++ {
                        if vectorMask.AtVec(int(j)) == 1 {
                            shares = append(shares, infectious.Share{j, vectorData[j]})
                        }
                    }

                    // Attempt rebuild
                    err := fec.Rebuild(shares, rebuiltSharesOutput)
                    if err == nil { // repair successful
                        progressMade = true

                        // Make backup of square
                        edsBackup, _ := eds.deepCopy()

                        // Insert rebuilt shares into square
                        for _, s := range rebuiltShares {
                            if mode == row {
                                eds.setCell(i, uint(s.Number), s.Data)
                            } else if mode == column {
                                eds.setCell(uint(s.Number), i, s.Data)
                            }
                        }

                        // Rebuild extended part if incomplete
                        if !vecSliceIsTrue(vectorMask, int(eds.originalDataWidth), int(eds.width)) {
                            if mode == row {
                                err = fec.Encode(flattenChunks(eds.rowSlice(i, 0, eds.originalDataWidth)), rebuiltExtendedSharesOutput)
                            } else if mode == column {
                                err = fec.Encode(flattenChunks(eds.columnSlice(0, i, eds.originalDataWidth)), rebuiltExtendedSharesOutput)
                            }
                            if err != nil {
                                return err
                            }
                            for _, s := range rebuiltShares {
                                if mode == row {
                                    eds.setCell(i, uint(s.Number), s.Data)
                                } else if mode == column {
                                    eds.setCell(uint(s.Number), i, s.Data)
                                }
                            }
                        }

                        // Check that rebuilt vector matches given merkle root
                        if mode == row {
                            if !bytes.Equal(eds.RowRoots()[i], rowRoots[i]) {
                                return &ByzantineRowError{i, edsBackup}
                            }
                        } else if mode == column {
                            if !bytes.Equal(eds.ColumnRoots()[i], columnRoots[i]) {
                                return &ByzantineColumnError{i, edsBackup}
                            }
                        }

                        // Check that newly completed orthogonal vectors match their new merkle roots
                        for j := uint(0); j < eds.width; j++ {
                            if vectorMask.AtVec(int(j)) == 0 {
                                if mode == row {
                                    adjMask := mask.ColView(int(j))
                                    if vecNumTrue(adjMask) == adjMask.Len()-1 && !bytes.Equal(eds.ColumnRoots()[j], columnRoots[j]) {
                                        return &ByzantineColumnError{j, edsBackup}
                                    }
                                } else if mode == column {
                                    adjMask := mask.RowView(int(j))
                                    if vecNumTrue(adjMask) == adjMask.Len()-1 && !bytes.Equal(eds.RowRoots()[j], rowRoots[j]) {
                                        return &ByzantineRowError{j, edsBackup}
                                    }
                                }
                            }
                        }

                        // Set vector mask to true
                        if mode == row {
                            for j := 0; j < int(eds.width); j++ {
                                mask.Set(int(i), j, 1)
                            }
                        } else if mode == column {
                            for j := 0; j < int(eds.width); j++ {
                                mask.Set(j, int(i), 1)
                            }
                        }
                    } else { // repair unsuccessful
                        solved = false
                    }
                }
            }
        }

        if solved {
            break
        } else if !progressMade {
            return &UnrepairableDataSquareError{}
        }
    }

    return nil
}

func (eds *ExtendedDataSquare) prerepairSanityCheck(rowRoots [][]byte, columnRoots [][]byte, mask mat.Dense) error {
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

    for i := uint(0); i < eds.width; i++ {
        rowMask := mask.RowView(int(i))
        columnMask := mask.ColView(int(i))
        if (vecIsTrue(rowMask) && !bytes.Equal(rowRoots[i], eds.RowRoots()[i])) || (vecIsTrue(columnMask) && !bytes.Equal(columnRoots[i], eds.ColumnRoots()[i])) {
            return errors.New("bad roots input")
        }

        if vecIsTrue(rowMask) {
            err = fec.Encode(flattenChunks(eds.rowSlice(i, 0, eds.originalDataWidth)), output)
            if err != nil {
                return err
            }
            if !reflect.DeepEqual(shares, eds.rowSlice(i, eds.originalDataWidth, eds.originalDataWidth)) {
                return &ByzantineRowError{i, *eds}
            }
        }

        if vecIsTrue(columnMask) {
            err = fec.Encode(flattenChunks(eds.columnSlice(0, i, eds.originalDataWidth)), output)
            if err != nil {
                return err
            }
            if !reflect.DeepEqual(shares, eds.columnSlice(eds.originalDataWidth, i, eds.originalDataWidth)) {
                return &ByzantineColumnError{i, *eds}
            }
        }
    }

    return nil
}

func vecIsTrue(vec mat.Vector) bool {
    for i := 0; i < vec.Len(); i++ {
        if vec.AtVec(i) == 0 {
            return false
        }
    }

    return true
}

func vecSliceIsTrue(vec mat.Vector, start int, end int) bool {
    for i := start; i < end; i++ {
        if vec.AtVec(i) == 0 {
            return false
        }
    }

    return true
}

func vecNumTrue(vec mat.Vector) int {
    var counter int
    for i := 0; i < vec.Len(); i++ {
        if vec.AtVec(i) == 1 {
            counter++
        }
    }

    return counter
}
