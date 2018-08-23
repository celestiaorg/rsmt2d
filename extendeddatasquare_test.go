package rsmt2d

import (
    "testing"
    "reflect"
)

func TestComputeExtendedDataSquare(t *testing.T) {
    for codec, _ := range SupportedCodecs {
        result, err := ComputeExtendedDataSquare([][]byte{
            {1}, {2},
            {3}, {4},
        }, codec)
        if (err != nil) {
            panic(err)
        }
        if (!reflect.DeepEqual(result.square, [][][]byte{
            {{1}, {2}, {7}, {13}},
            {{3}, {4}, {13}, {31}},
            {{5}, {14}, {19}, {41}},
            {{9}, {26}, {47}, {69}},
        })) {
            t.Errorf("NewExtendedDataSquare failed for 2x2 square with chunk size 1")
        }
    }
}
