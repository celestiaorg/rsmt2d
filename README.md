# rsmt2d

Go implementation of [two dimensional Reed-Solomon Merkle tree data availability scheme](https://arxiv.org/abs/1809.09044).

[![Tests](https://github.com/celestiaorg/rsmt2d/actions/workflows/ci.yml/badge.svg)](https://github.com/celestiaorg/rsmt2d/actions/workflows/ci.yml)
[![Codecov](https://img.shields.io/codecov/c/github/celestiaorg/rsmt2d)](https://app.codecov.io/gh/celestiaorg/rsmt2d)
[![GoDoc](https://godoc.org/github.com/celestiaorg/rsmt2d?status.svg)](https://godoc.org/github.com/celestiaorg/rsmt2d)

## Example

```go
package main

import (
    "bytes"

    "github.com/celestiaorg/rsmt2d"
)

func main() {
    // Size of each share, in bytes
    bufferSize := 64
    // Init new codec
    codec := rsmt2d.NewLeoRSCodec()

    ones := bytes.Repeat([]byte{1}, bufferSize)
    twos := bytes.Repeat([]byte{2}, bufferSize)
    threes := bytes.Repeat([]byte{3}, bufferSize)
    fours := bytes.Repeat([]byte{4}, bufferSize)

    // Compute parity shares
    eds, err := rsmt2d.ComputeExtendedDataSquare(
        [][]byte{
            ones, twos,
            threes, fours,
        },
        codec,
        rsmt2d.NewDefaultTree,
    )
    if err != nil {
        // ComputeExtendedDataSquare failed
    }

    flattened := eds.Flattened()

    // Delete some shares, just enough so that repairing is possible.
    flattened[0], flattened[2], flattened[3] = nil, nil, nil
    flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
    flattened[8], flattened[9], flattened[10] = nil, nil, nil
    flattened[12], flattened[13] = nil, nil

    // Re-import the data square.
    eds, err = rsmt2d.ImportExtendedDataSquare(flattened, codec, rsmt2d.NewDefaultTree)
    if err != nil {
        // ImportExtendedDataSquare failed
    }

    // Repair square.
    err := eds.Repair(
        eds.RowRoots(),
        eds.ColRoots(),
    )
    if err != nil {
        // err contains information to construct a fraud proof
        // See extendeddatacrossword_test.go
    }
}
```

## Contributing

1. [Install Go](https://go.dev/doc/install) 1.20+
1. [Install golangci-lint](https://golangci-lint.run/usage/install/)

### Helpful Commands

```sh
# Run unit tests
go test ./...

# Run benchmarks
go test -benchmem -bench=.

# Run linter
golangci-lint run
```
