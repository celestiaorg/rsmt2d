# rsmt2d

Go and Rust implementation of [two dimensional Reed-Solomon Merkle tree data availability scheme](https://arxiv.org/abs/1809.09044).

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
    // shareSize is the size of each share (in bytes).
    shareSize := 512
    // Init new codec
    codec := rsmt2d.NewLeoRSCodec()

    ones := bytes.Repeat([]byte{1}, shareSize)
    twos := bytes.Repeat([]byte{2}, shareSize)
    threes := bytes.Repeat([]byte{3}, shareSize)
    fours := bytes.Repeat([]byte{4}, shareSize)

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

    rowRoots, err := eds.RowRoots()
    if err != nil {
	// RowRoots failed
    }
    colRoots, err := eds.ColRoots()
    if err != nil {
	// ColRoots failed
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
    err = eds.Repair(
        rowRoots,
        colRoots,
    )
    if err != nil {
        // err contains information to construct a fraud proof
        // See extendeddatacrossword_test.go
    }
}
```

## Rust Implementation

This repository contains both Go and Rust implementations of the rsmt2d library. The Rust implementation provides the same core functionality as the Go version:

- Data square construction and manipulation
- Reed-Solomon encoding/decoding (placeholder implementation)
- Merkle tree operations (placeholder implementation)
- Extended data square with repair functionality (placeholder implementation)

The Rust implementation is currently in early development with placeholder implementations for Reed-Solomon codec and repair algorithms. This provides a foundation for a complete Rust port while maintaining the existing Go functionality.

### Rust Example

```rust
use rsmt2d::{ExtendedDataSquare, LeoRSCodec, new_default_tree};

// Create some example data
let data = vec![
    vec![1, 2], vec![3, 4],
    vec![5, 6], vec![7, 8],
];

// Import as extended data square
let codec = Box::new(LeoRSCodec::new());
let mut eds = ExtendedDataSquare::import(data, codec, new_default_tree)?;

// Get roots (when implementations are complete)
let row_roots = eds.row_roots()?;
let col_roots = eds.col_roots()?;
```

## Contributing

### Go Development

1. [Install Go](https://go.dev/doc/install) 1.21+
1. [Install golangci-lint](https://golangci-lint.run/usage/install/)

### Rust Development

1. [Install Rust](https://rustup.rs/) 1.86+
1. Run `cargo --version` to verify installation

### Helpful Commands

#### Go Commands

```sh
# Run unit tests
go test ./...

# Run benchmarks
go test -benchmem -bench=.

# Run linter
golangci-lint run
```

#### Rust Commands

```sh
# Run unit tests
cargo test

# Run benchmarks
cargo bench

# Run linter
cargo clippy

# Build
cargo build
```

## Audits

[Informal Systems](https://informal.systems/) audited rsmt2d [v0.9.0](https://github.com/celestiaorg/rsmt2d/releases/tag/v0.9.0) in Q2 of 2023. See [informal-systems.pdf](./audit/informal-systems.pdf).
