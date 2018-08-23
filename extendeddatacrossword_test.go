package rsmt2d

import (
    "testing"
    "reflect"
)

func TestRepairExtendedDataSquare(t *testing.T) {
    for codec, _ := range SupportedCodecs {
        original, err := ComputeExtendedDataSquare([][]byte{
            {1}, {2},
            {3}, {4},
        }, codec)
        if (err != nil) {
            panic(err)
        }

        flattened := original.flattened()
        flattened[0], flattened[2], flattened[3] = nil, nil, nil
        flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
        flattened[8], flattened[9], flattened[10] = nil, nil, nil
        flattened[12], flattened[13] = nil, nil
        var result *ExtendedDataSquare
        result, err = RepairExtendedDataSquare(original.RowRoots(), original.ColumnRoots(), flattened, codec)
        if (!reflect.DeepEqual(result.square, [][][]byte{
            {{1}, {2}, {7}, {13}},
            {{3}, {4}, {13}, {31}},
            {{5}, {14}, {19}, {41}},
            {{9}, {26}, {47}, {69}},
        })) {
            t.Errorf("failed to repair a repairable square")
        }

        flattened = original.flattened()
        flattened[0], flattened[2], flattened[3] = nil, nil, nil
        flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
        flattened[8], flattened[9], flattened[10] = nil, nil, nil
        flattened[12], flattened[13], flattened[14] = nil, nil, nil
        result, err = RepairExtendedDataSquare(original.RowRoots(), original.ColumnRoots(), flattened, codec)
        if err == nil {
            t.Errorf("did not return an error on trying to repair an unrepairable square")
        }

        var corrupted ExtendedDataSquare
        corrupted, err = original.deepCopy()
        corrupted.setCell(0, 0, []byte{66})
        result, err = RepairExtendedDataSquare(original.RowRoots(), original.ColumnRoots(), corrupted.flattened(), codec)
        if err == nil {
            t.Errorf("did not return an error on trying to repair a square with bad roots")
        }

        var ok bool
        corrupted, err = original.deepCopy()
        corrupted.setCell(0, 0, []byte{66})
        result, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), corrupted.flattened(), codec)
        if err, ok = err.(*ByzantineRowError); !ok {
            t.Errorf("did not return a ByzantineRowError for a bad row")
        }

        corrupted, err = original.deepCopy()
        corrupted.setCell(0, 3, []byte{66})
        result, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), corrupted.flattened(), codec)
        if err, ok = err.(*ByzantineRowError); !ok {
            t.Errorf("did not return a ByzantineRowError for a bad row")
        }

        corrupted, err = original.deepCopy()
        corrupted.setCell(0, 0, []byte{66})
        flattened = corrupted.flattened()
        flattened[1], flattened[2], flattened[3] = nil, nil, nil
        result, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), flattened, codec)
        if err, ok = err.(*ByzantineColumnError); !ok {
            t.Errorf("did not return a ByzantineColumnError for a bad column")
        }

        corrupted, err = original.deepCopy()
        corrupted.setCell(3, 0, []byte{66})
        flattened = corrupted.flattened()
        flattened[1], flattened[2], flattened[3] = nil, nil, nil
        result, err = RepairExtendedDataSquare(corrupted.RowRoots(), corrupted.ColumnRoots(), flattened, codec)
        if err, ok = err.(*ByzantineColumnError); !ok {
            t.Errorf("did not return a ByzantineColumnError for a bad column")
        }
    }
}
