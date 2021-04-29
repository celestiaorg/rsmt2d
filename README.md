# rsmt2d

Go implementation of [two dimensional Reed-Solomon Merkle tree data availability scheme](https://arxiv.org/abs/1809.09044).

[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/lazyledger/rsmt2d/Tests)](https://github.com/lazyledger/rsmt2d/actions/workflows/ci.yml)
[![Codecov](https://img.shields.io/codecov/c/github/lazyledger/rsmt2d)](https://app.codecov.io/gh/lazyledger/rsmt2d)
[![GoDoc](https://godoc.org/github.com/lazyledger/rsmt2d?status.svg)](https://godoc.org/github.com/lazyledger/rsmt2d)

## Example

```go
package main

import (
    "bytes"

    "github.com/lazyledger/rsmt2d"
)

func main() {
    // Size of each share, in bytes
    bufferSize := 64
    // Init new codec
    codec := rsmt2d.NewLeoRSFF8Codec()

    ones := bytes.Repeat([]byte{1}, bufferSize)
    twos := bytes.Repeat([]byte{2}, bufferSize)
    threes := bytes.Repeat([]byte{3}, bufferSize)
    fours := bytes.Repeat([]byte{4}, bufferSize)

    // Compute parity shares
    eds, _ := rsmt2d.ComputeExtendedDataSquare(
        [][]byte{
            ones, twos,
            threes, fours,
        },
        codec,
        rsmt2d.NewDefaultTree,
    )

    // Save all shares in flattended form.
    // Note: slices returned from Row() an Column() are read-only.
    // If you need to write to them, copy first.
    flattened := make([][]byte, 0, eds.Width()*eds.Width())
    for i := uint(0); i < eds.Width(); i++ {
        startIndex := i * eds.Width()
        copy(flattened[startIndex:startIndex+eds.Width()], eds.Row(i))
    }

    // Delete some shares, just enough so that repairing is possible.
    flattened[0], flattened[2], flattened[3] = nil, nil, nil
    flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
    flattened[8], flattened[9], flattened[10] = nil, nil, nil
    flattened[12], flattened[13] = nil, nil

    // Repair square.
    repaired, err := rsmt2d.RepairExtendedDataSquare(
        eds.RowRoots(),
        eds.ColumnRoots(),
        flattened,
        codec,
        rsmt2d.NewDefaultTree,
    )
    if err != nil {
        // err contains information to construct a fraud proof
        // See extendeddatacrossword_test.go
    }
    _ = repaired
}

```

## Building From Source

Run benchmarks

```sh
go test -tags leopard -benchmem -bench=.
```
