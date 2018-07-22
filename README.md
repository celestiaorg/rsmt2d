# rsmt2d
Go implementation of two dimensional Reed-Solomon merkle tree data availability scheme

[![Build Status](https://img.shields.io/travis/musalbas/rsmt2d.svg)](https://travis-ci.org/musalbas/rsmt2d)
[![Coverage Status](https://img.shields.io/coveralls/github/musalbas/rsmt2d.svg)](https://coveralls.io/github/musalbas/rsmt2d?branch=master)
[![GoDoc](https://godoc.org/github.com/musalbas/rsmt2d?status.svg)](https://godoc.org/github.com/musalbas/rsmt2d)

Experimental software. Use at your own risk! May have security flaws.

## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [type ByzantineColumnError](#ByzantineColumnError)
  * [func (e *ByzantineColumnError) Error() string](#ByzantineColumnError.Error)
* [type ByzantineRowError](#ByzantineRowError)
  * [func (e *ByzantineRowError) Error() string](#ByzantineRowError.Error)
* [type ExtendedDataSquare](#ExtendedDataSquare)
  * [func ComputeExtendedDataSquare(data [][]byte) (*ExtendedDataSquare, error)](#ComputeExtendedDataSquare)
  * [func ImportExtendedDataSquare(data [][]byte) (*ExtendedDataSquare, error)](#ImportExtendedDataSquare)
  * [func RepairExtendedDataSquare(rowRoots [][]byte, columnRoots [][]byte, data [][]byte) (*ExtendedDataSquare, error)](#RepairExtendedDataSquare)
  * [func (ds ExtendedDataSquare) ColumnRoots() [][]byte](#ExtendedDataSquare.ColumnRoots)
  * [func (ds ExtendedDataSquare) RowRoots() [][]byte](#ExtendedDataSquare.RowRoots)
* [type UnrepairableDataSquareError](#UnrepairableDataSquareError)
  * [func (e *UnrepairableDataSquareError) Error() string](#UnrepairableDataSquareError.Error)


#### <a name="pkg-files">Package files</a>
[datasquare.go](/src/github.com/musalbas/rsmt2d/datasquare.go) [extendeddatacrossword.go](/src/github.com/musalbas/rsmt2d/extendeddatacrossword.go) [extendeddatasquare.go](/src/github.com/musalbas/rsmt2d/extendeddatasquare.go) [utils.go](/src/github.com/musalbas/rsmt2d/utils.go) 


## <a name="pkg-constants">Constants</a>
``` go
const MaxChunks = 128 * 128 // Using Galois Field 256 correcting up to t/2 symbols

```
The max number of original data chunks.





## <a name="ByzantineColumnError">type</a> [ByzantineColumnError](/src/target/extendeddatacrossword.go?s=596:692#L29)
``` go
type ByzantineColumnError struct {
    ColumnNumber   uint
    LastGoodSquare ExtendedDataSquare
}
```
ByzantineColumnError is thrown when there is a repaired column does not match the expected column merkle root.










### <a name="ByzantineColumnError.Error">func</a> (\*ByzantineColumnError) [Error](/src/target/extendeddatacrossword.go?s=694:739#L34)
``` go
func (e *ByzantineColumnError) Error() string
```



## <a name="ByzantineRowError">type</a> [ByzantineRowError](/src/target/extendeddatacrossword.go?s=285:375#L19)
``` go
type ByzantineRowError struct {
    RowNumber      uint
    LastGoodSquare ExtendedDataSquare
}
```
ByzantineRowError is thrown when there is a repaired row does not match the expected row merkle root.










### <a name="ByzantineRowError.Error">func</a> (\*ByzantineRowError) [Error](/src/target/extendeddatacrossword.go?s=377:419#L24)
``` go
func (e *ByzantineRowError) Error() string
```



## <a name="ExtendedDataSquare">type</a> [ExtendedDataSquare](/src/target/extendeddatasquare.go?s=374:451#L15)
``` go
type ExtendedDataSquare struct {
    // contains filtered or unexported fields
}
```
ExtendedDataSquare represents an extended piece of data.







### <a name="ComputeExtendedDataSquare">func</a> [ComputeExtendedDataSquare](/src/target/extendeddatasquare.go?s=541:615#L21)
``` go
func ComputeExtendedDataSquare(data [][]byte) (*ExtendedDataSquare, error)
```
ComputeExtendedDataSquare computes the extended data square for some chunks of data.


### <a name="ImportExtendedDataSquare">func</a> [ImportExtendedDataSquare](/src/target/extendeddatasquare.go?s=1074:1147#L41)
``` go
func ImportExtendedDataSquare(data [][]byte) (*ExtendedDataSquare, error)
```
ImportExtendedDataSquare imports an extended data square, represented as flattened chunks of data.


### <a name="RepairExtendedDataSquare">func</a> [RepairExtendedDataSquare](/src/target/extendeddatacrossword.go?s=1224:1338#L48)
``` go
func RepairExtendedDataSquare(rowRoots [][]byte, columnRoots [][]byte, data [][]byte) (*ExtendedDataSquare, error)
```
RepairExtendedDataSquare repairs an incomplete extended data square, against its expected row and column merkle roots.
Missing data chunks should be represented as nil.





### <a name="ExtendedDataSquare.ColumnRoots">func</a> (ExtendedDataSquare) [ColumnRoots](/src/target/datasquare.go?s=4200:4244#L172)
``` go
func (ds ExtendedDataSquare) ColumnRoots() [][]byte
```



### <a name="ExtendedDataSquare.RowRoots">func</a> (ExtendedDataSquare) [RowRoots](/src/target/datasquare.go?s=4069:4110#L164)
``` go
func (ds ExtendedDataSquare) RowRoots() [][]byte
```



## <a name="UnrepairableDataSquareError">type</a> [UnrepairableDataSquareError](/src/target/extendeddatacrossword.go?s=905:948#L39)
``` go
type UnrepairableDataSquareError struct {
}
```
UnrepairableDataSquareError is thrown when there is insufficient chunks to repair the square.










### <a name="UnrepairableDataSquareError.Error">func</a> (\*UnrepairableDataSquareError) [Error](/src/target/extendeddatacrossword.go?s=950:1002#L42)
``` go
func (e *UnrepairableDataSquareError) Error() string
```
