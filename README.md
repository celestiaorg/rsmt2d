# rsmt2d

Go implementation of [two dimensional Reed-Solomon Merkle tree data availability scheme](https://arxiv.org/abs/1809.09044).

[![Tests](https://github.com/celestiaorg/rsmt2d/actions/workflows/ci.yml/badge.svg)](https://github.com/celestiaorg/rsmt2d/actions/workflows/ci.yml)
[![Codecov](https://img.shields.io/codecov/c/github/celestiaorg/rsmt2d/main)](https://app.codecov.io/gh/celestiaorg/rsmt2d)
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

## Contributing

PR titles follow [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) and must start the subject with `[high]`, `[medium]`, or `[low]` to indicate the review thoroughness needed. For example: `fix!: [high] correct repair of byzantine rows` or `docs: [low] clarify codec usage`. Include the same tag in the PR description and explain why you chose it.

- `[high]`: Deep review for consensus, security, or other changes with significant risk, e.g. erasure coding/codecs, extended data square construction, repair, or root computation.
- `[medium]`: Standard review for code changes with limited impact and well-understood behavior.
- `[low]`: Focused review for straightforward docs, tests, or tooling changes with low risk.

Choose the higher level when unsure. A small diff can still need deep review. CI checks the title format; reviewers should confirm the chosen level. Dependabot defaults to `[high]` for Go dependencies and `[medium]` for GitHub Actions.

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

# Run each fuzz target for 30s (override with FUZZTIME=5m)
make fuzz

# Run linter
make lint
```

## Audits

[Informal Systems](https://informal.systems/) audited rsmt2d [v0.9.0](https://github.com/celestiaorg/rsmt2d/releases/tag/v0.9.0) in Q2 of 2023. See [informal-systems.pdf](./audit/informal-systems.pdf).
