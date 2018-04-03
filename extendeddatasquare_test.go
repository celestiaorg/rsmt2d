package rsmt2d

import (
    "testing"
)

func TestNewExtendedDataSquare(t *testing.T) {
    result, err := NewExtendedDataSquare([][]byte{{1, 2}, {3, 4}, {5, 6}, {7, 8}})
    if (err != nil) {
        panic(err)
    }
    t.Log(result.square)
}
