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

## Trees and JSON serialization

rsmt2d has two built-in Merkle trees:

- `rsmt2d.NMTTreeName` (`"nmt"`): Celestia's erasured namespaced Merkle tree
  (sha256, namespace size 29, `ignoreMaxNamespace=true`), root-compatible
  with celestia-app's `wrapper.ErasuredNamespacedMerkleTree`. This is the
  default tree for JSON deserialization.
- `rsmt2d.DefaultTreeName` (`"default-tree"`): a plain sha256 Merkle tree,
  retained as a test fixture and for legacy compatibility.

`ExtendedDataSquare.MarshalJSON` records the tree name and `UnmarshalJSON`
uses it to reconstruct the same tree, so row and column roots survive a JSON
round trip. Build squares with the `WithTree` constructors to record the
tree name explicitly:

```go
eds, err := rsmt2d.ComputeExtendedDataSquareWithTree(data, codec, rsmt2d.NMTTreeName)
```

Squares built from a raw `TreeConstructorFn` (via `ComputeExtendedDataSquare`
or `ImportExtendedDataSquare`) carry no tree name and marshal without a
`tree` field. JSON without a `tree` field — including all JSON produced by
older versions of this library — is deserialized with the NMT tree, since
every square serialized by production Celestia software is NMT-built.

## Contributing

1. [Install Go](https://go.dev/doc/install) 1.24+
1. [Install golangci-lint](https://golangci-lint.run/usage/install/)

### Helpful Commands

```sh
# Build the project
make build

# Run unit tests
make test

# Run benchmarks
make bench

# Run linter
make lint
```

## Audits

[Informal Systems](https://informal.systems/) audited rsmt2d [v0.9.0](https://github.com/celestiaorg/rsmt2d/releases/tag/v0.9.0) in Q2 of 2023. See [informal-systems.pdf](./audit/informal-systems.pdf).
