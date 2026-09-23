package rsmt2d_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/rsmt2d"
	"github.com/stretchr/testify/require"
)

func TestExportedSlicesAreIndependent(t *testing.T) {
	for _, name := range []string{"Flattened", "FlattenedODS", "Roots"} {
		t.Run(name, func(t *testing.T) {
			eds := createExampleEds(t, shareSize)
			original := eds.Flattened()
			originalRoots, err := eds.Roots()
			require.NoError(t, err)
			read := func() [][]byte {
				switch name {
				case "Flattened":
					return eds.Flattened()
				case "FlattenedODS":
					return eds.FlattenedODS()
				default:
					roots, err := eds.Roots()
					require.NoError(t, err)
					return roots
				}
			}
			want := read()
			got := read()
			got[0][0] ^= 0xff
			got[1] = nil
			require.Equal(t, want, read())
			require.Equal(t, original, eds.Flattened())
			roots, err := eds.Roots()
			require.NoError(t, err)
			require.Equal(t, originalRoots, roots)
		})
	}
}

func TestRepairRetriesOnSameSquare(t *testing.T) {
	original := createExampleEds(t, shareSize)
	rowRoots, err := original.RowRoots()
	require.NoError(t, err)
	colRoots, err := original.ColRoots()
	require.NoError(t, err)
	eds, err := rsmt2d.NewExtendedDataSquare(rsmt2d.NewLeoRSCodec(), rsmt2d.NewDefaultTree, original.Width(), shareSize)
	require.NoError(t, err)
	// Three original shares allow some progress, but cannot recover the square.
	for _, cell := range [][2]uint{{0, 0}, {0, 1}, {1, 0}} {
		require.NoError(t, eds.SetCell(cell[0], cell[1], original.GetCell(cell[0], cell[1])))
	}
	require.ErrorIs(t, eds.Repair(rowRoots, colRoots), rsmt2d.ErrUnrepairableDataSquare)
	require.NotNil(t, eds.GetCell(0, 2), "the failed attempt should have made progress")
	require.Nil(t, eds.GetCell(1, 1))
	require.NoError(t, eds.SetCell(1, 1, original.GetCell(1, 1)))
	require.NoError(t, eds.Repair(rowRoots, colRoots))
	require.Equal(t, original.Flattened(), eds.Flattened())
	repairedRows, err := eds.RowRoots()
	require.NoError(t, err)
	require.Equal(t, rowRoots, repairedRows)
	repairedCols, err := eds.ColRoots()
	require.NoError(t, err)
	require.Equal(t, colRoots, repairedCols)
}

func TestLeoRSCodecDecodeMutatesInput(t *testing.T) {
	codec := rsmt2d.NewLeoRSCodec()
	data := [][]byte{bytes.Repeat([]byte{1}, 64), bytes.Repeat([]byte{2}, 64)}
	parity, err := codec.Encode(data)
	require.NoError(t, err)
	want := make([][]byte, 0, len(data)+len(parity))
	want = append(want, data...)
	want = append(want, parity...)
	for _, missing := range [][]int{{0, 1}, {2, 3}, {0, 3}} {
		input := make([][]byte, len(want))
		for i := range want {
			input[i] = bytes.Clone(want[i])
		}
		for _, i := range missing {
			input[i] = nil
		}
		present := append([][]byte(nil), input...)
		decoded, err := codec.Decode(input)
		require.NoError(t, err)
		require.Equal(t, want, input, "Decode must fill the caller's missing entries")
		require.Equal(t, want, decoded)
		require.Same(t, &input[0], &decoded[0], "Decode must return the supplied slice")
		for i, share := range present {
			if share != nil {
				require.Same(t, &share[0], &input[i][0], "present share storage must be preserved")
				require.Equal(t, want[i], input[i])
			}
		}
	}
}
