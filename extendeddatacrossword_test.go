package rsmt2d

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// PseudoFraudProof is an example fraud proof.
// TODO a real fraud proof would have a Merkle proof for each share.
type PseudoFraudProof struct {
	Mode   int      // Row (0) or column (1)
	Index  uint     // Row or column index
	Shares [][]byte // Bad shares (nil are missing)
}

func TestRepairExtendedDataSquare(t *testing.T) {
	for codecName, codec := range codecs {

		bufferSize := 64
		ones := bytes.Repeat([]byte{1}, bufferSize)
		twos := bytes.Repeat([]byte{2}, bufferSize)
		threes := bytes.Repeat([]byte{3}, bufferSize)
		fours := bytes.Repeat([]byte{4}, bufferSize)

		original, err := ComputeExtendedDataSquare([][]byte{
			ones, twos,
			threes, fours,
		}, codec, NewDefaultTree)
		if err != nil {
			panic(err)
		}

		flattened := original.flattened()
		flattened[0], flattened[2], flattened[3] = nil, nil, nil
		flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
		flattened[8], flattened[9], flattened[10] = nil, nil, nil
		flattened[12], flattened[13] = nil, nil
		var result *ExtendedDataSquare
		result, err = RepairExtendedDataSquare(original.getRowRoots(), original.getColRoots(), flattened, codec, NewDefaultTree)
		if err != nil {
			t.Errorf("unexpected err while repairing data square: %v, codec: :%s", err, codecName)
		} else {
			assert.Equal(t, result.getCell(0, 0), ones)
			assert.Equal(t, result.getCell(0, 1), twos)
			assert.Equal(t, result.getCell(1, 0), threes)
			assert.Equal(t, result.getCell(1, 1), fours)
		}

		flattened = original.flattened()
		flattened[0], flattened[2], flattened[3] = nil, nil, nil
		flattened[4], flattened[5], flattened[6], flattened[7] = nil, nil, nil, nil
		flattened[8], flattened[9], flattened[10] = nil, nil, nil
		flattened[12], flattened[13], flattened[14] = nil, nil, nil

		var eds *ExtendedDataSquare
		eds, err = RepairExtendedDataSquare(original.getRowRoots(), original.getColRoots(), flattened, codec, NewDefaultTree)
		if err == nil {
			t.Errorf("did not return an error on trying to repair an unrepairable square")
		}
		if eds == nil {
			t.Errorf("did not return a repaired EDS")
		}
		var corrupted ExtendedDataSquare
		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corruptChunk := bytes.Repeat([]byte{66}, bufferSize)
		corrupted.setCell(0, 0, corruptChunk)
		_, err = RepairExtendedDataSquare(original.getRowRoots(), original.getColRoots(), corrupted.flattened(), codec, NewDefaultTree)
		if err == nil {
			t.Errorf("did not return an error on trying to repair a square with bad roots")
		}

		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corrupted.setCell(0, 0, corruptChunk)
		eds, err = RepairExtendedDataSquare(corrupted.getRowRoots(), corrupted.getColRoots(), corrupted.flattened(), codec, NewDefaultTree)
		var byzRow *ErrByzantineRow
		if !errors.As(err, &byzRow) {
			t.Errorf("did not return a ErrByzantineRow for a bad row; got: %v", err)
		}
		if eds == nil {
			t.Errorf("did not return a repaired EDS")
		}
		// Construct the fraud proof
		fraudProof := PseudoFraudProof{row, byzRow.RowNumber, byzRow.Shares}
		// Verify the fraud proof
		// TODO in a real fraud proof, also verify Merkle proof for each non-nil share.
		rebuiltShares, err := codec.Decode(fraudProof.Shares)
		if err != nil {
			t.Errorf("could not decode fraud proof shares; got: %v", err)
		}
		root := corrupted.computeSharesRoot(rebuiltShares, fraudProof.Index)
		if bytes.Equal(root, corrupted.getRowRoot(fraudProof.Index)) {
			// If the roots match, then the fraud proof should be for invalid erasure coding.
			parityShares, err := codec.Encode(rebuiltShares[0:corrupted.originalDataWidth])
			if err != nil {
				t.Errorf("could not encode fraud proof shares; %v", fraudProof)
			}
			startIndex := len(rebuiltShares) - int(corrupted.originalDataWidth)
			if bytes.Equal(flattenChunks(parityShares), flattenChunks(rebuiltShares[startIndex:])) {
				t.Errorf("invalid fraud proof %v", fraudProof)
			}
		}

		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corrupted.setCell(0, 3, corruptChunk)
		eds, err = RepairExtendedDataSquare(corrupted.getRowRoots(), corrupted.getColRoots(), corrupted.flattened(), codec, NewDefaultTree)
		if !errors.As(err, &byzRow) {
			t.Errorf("did not return a ErrByzantineRow for a bad row; got %v", err)
		}
		if eds == nil {
			t.Errorf("did not return a repaired EDS")
		}

		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corrupted.setCell(0, 0, corruptChunk)
		flattened = corrupted.flattened()
		flattened[1], flattened[2], flattened[3] = nil, nil, nil
		eds, err = RepairExtendedDataSquare(corrupted.getRowRoots(), corrupted.getColRoots(), flattened, codec, NewDefaultTree)
		var byzCol *ErrByzantineCol
		if !errors.As(err, &byzCol) {
			t.Errorf("did not return a ErrByzantineCol for a bad column; got %v", err)
		}
		if eds == nil {
			t.Errorf("did not return a repaired EDS")
		}
		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corrupted.setCell(3, 0, corruptChunk)
		flattened = corrupted.flattened()
		flattened[1], flattened[2], flattened[3] = nil, nil, nil
		eds, err = RepairExtendedDataSquare(corrupted.getRowRoots(), corrupted.getColRoots(), flattened, codec, NewDefaultTree)
		if !errors.As(err, &byzCol) {
			t.Errorf("did not return a ErrByzantineCol for a bad column; got %v", err)
		}
		if eds == nil {
			t.Errorf("did not return a repaired EDS")
		}

		// Additional case to check that repaired eds is the same as after solvedCrossword
		corrupted, err = original.deepCopy(codec)
		if err != nil {
			t.Fatalf("unexpected err while copying original data: %v, codec: :%s", err, codecName)
		}
		corrupted.setCell(3, 0, corruptChunk)
		flattened = corrupted.flattened()
		flattened[1], flattened[2], flattened[3] = nil, nil, nil
		eds, err = RepairExtendedDataSquare(corrupted.getRowRoots(), corrupted.getColRoots(), flattened, codec, NewDefaultTree)
		if err == nil {
			t.Fatalf("expected error did no received")
		}
		if eds == nil {
			t.Fatalf("did not return a repaired EDS")
		}
		// Prepare data for solving
		width := int(math.Ceil(math.Sqrt(float64(len(flattened)))))
		bitMat := newBitMatrix(width)
		var chunkSize int
		for i := range flattened {
			if flattened[i] != nil {
				bitMat.SetFlat(i)
				if chunkSize == 0 {
					chunkSize = len(flattened[i])
				}
			}
		}
		edsNew, err := ImportExtendedDataSquare(flattened, codec, NewDefaultTree)
		if err != nil {
			t.Errorf("did not return a repaired EDS")
		}
		_ = edsNew.solveCrossword(corrupted.getRowRoots(), corrupted.getColRoots(), bitMat, codec)

		if !reflect.DeepEqual(edsNew.ColRoots(), eds.ColRoots()) {
			t.Fatal("col roots of EDSes do not match: eds after solving: ", edsNew.ColRoots(), ",eds after repairing: ", eds.ColRoots())
		}
		if !reflect.DeepEqual(edsNew.RowRoots(), eds.RowRoots()) {
			t.Fatal("row roots of EDSes do not match: eds after solving: ", edsNew.RowRoots(), ", eds after repairing: ", eds.RowRoots())
		}
	}
}

func BenchmarkRepair(b *testing.B) {
	// For different ODS sizes
	for originalDataWidth := 16; originalDataWidth <= 128; originalDataWidth *= 2 {
		for codecName, codec := range codecs {
			// Generate a new range original data square then extend it
			square := genRandDS(originalDataWidth)
			eds, err := ComputeExtendedDataSquare(square, codec, NewDefaultTree)
			if err != nil {
				b.Error(err)
			}

			extendedDataWidth := originalDataWidth * 2

			b.Run(
				fmt.Sprintf(
					"Repairing %dx%d ODS using %s",
					originalDataWidth,
					originalDataWidth,
					codecName,
				),
				func(b *testing.B) {
					for n := 0; n < b.N; n++ {
						b.StopTimer()

						flattened := eds.flattened()
						// Randomly remove 1/2 of the shares of each row
						for r := 0; r < extendedDataWidth; r++ {
							for c := 0; c < originalDataWidth; {
								ind := rand.Intn(extendedDataWidth)
								if flattened[r*extendedDataWidth+ind] == nil {
									continue
								}
								flattened[r*extendedDataWidth+ind] = nil
								c++
							}
						}

						b.StartTimer()

						_, err := RepairExtendedDataSquare(
							eds.getRowRoots(),
							eds.getColRoots(),
							flattened,
							codec,
							NewDefaultTree,
						)
						if err != nil {
							b.Error(err)
						}
					}
				},
			)
		}
	}
}
