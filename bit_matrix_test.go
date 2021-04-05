package rsmt2d

import (
	"testing"
)

func Test_bitMatrix_ColRangeIsOne(t *testing.T) {
	type initParams struct {
		squareSize int
		bits       int
	}
	type colRange struct {
		c     int
		start int
		end   int
	}
	tests := []struct {
		name          string
		initParams    initParams
		setBits       []int
		rangeInColumn colRange
		want          bool
	}{
		{"empty", initParams{4, 16}, nil, colRange{0, 0, 4}, false},
		{"empty", initParams{4, 16}, nil, colRange{1, 0, 4}, false},
		{"empty", initParams{4, 16}, nil, colRange{2, 0, 4}, false},
		{"empty", initParams{4, 16}, nil, colRange{3, 0, 4}, false},

		{"none set in requested range", initParams{4, 16}, []int{0, 1, 2, 3, 4, 5, 6}, colRange{0, 0, 4}, false},
		{"none set in requested range", initParams{4, 16}, []int{0, 4}, colRange{0, 1, 3}, false},

		{"all set in requested range", initParams{4, 16}, []int{0, 4, 8}, colRange{0, 0, 1}, true},
		{"all set in requested range", initParams{4, 16}, []int{0, 4, 8}, colRange{0, 0, 2}, true},
		{"all set in requested range", initParams{4, 16}, []int{0, 4, 8, 16}, colRange{0, 0, 1}, true},
		{"all set in requested range", initParams{4, 16}, []int{0, 4, 8, 16}, colRange{0, 0, 2}, true},
		{"all set in requested range", initParams{4, 16}, []int{0, 4, 8, 12}, colRange{0, 0, 3}, true},
		{"all set in requested range", initParams{4, 16}, []int{1, 5, 9, 13}, colRange{1, 0, 3}, true},
		{"all set in requested range", initParams{4, 16}, []int{2, 6, 10, 14}, colRange{2, 0, 3}, true},
		{"all set in requested range", initParams{4, 16}, []int{3, 7, 11, 15}, colRange{3, 0, 3}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := newBitMatrix(tt.initParams.squareSize, tt.initParams.bits)
			for _, flatIdx := range tt.setBits {
				bm.SetFlat(flatIdx)
			}
			if got := bm.ColRangeIsOne(tt.rangeInColumn.c, tt.rangeInColumn.start, tt.rangeInColumn.end); got != tt.want {
				t.Errorf("ColRangeIsOne() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bitMatrix_ColumnIsOne(t *testing.T) {
	type initParams struct {
		squareSize int
		bits       int
	}

	tests := []struct {
		name       string
		initParams initParams
		setBits    []int
		column     int
		want       bool
	}{
		{"empty", initParams{4, 16}, []int{}, 0, false},
		{"empty", initParams{4, 16}, []int{}, 1, false},
		{"empty", initParams{4, 16}, []int{}, 2, false},
		{"empty", initParams{4, 16}, []int{}, 3, false},

		{"col_0 == 1", initParams{4, 16}, []int{0, 4, 8, 12}, 0, true},
		{"col_1 != 1", initParams{4, 16}, []int{0, 4, 8, 12}, 1, false},
		{"col_2 != 1", initParams{4, 16}, []int{0, 4, 8, 12}, 2, false},
		{"col_3 != 1", initParams{4, 16}, []int{0, 4, 8, 12}, 3, false},

		{"col_0 != 1", initParams{4, 16}, []int{1, 5, 9, 13}, 0, false},
		{"col_1 == 1", initParams{4, 16}, []int{1, 5, 9, 13}, 1, true},
		{"col_2 != 1", initParams{4, 16}, []int{1, 5, 9, 13}, 2, false},
		{"col_3 != 1", initParams{4, 16}, []int{1, 5, 9, 13}, 3, false},

		{"col_0 != 1", initParams{4, 16}, []int{2, 6, 10, 14}, 0, false},
		{"col_1 != 1", initParams{4, 16}, []int{2, 6, 10, 14}, 1, false},
		{"col_2 == 1", initParams{4, 16}, []int{2, 6, 10, 14}, 2, true},
		{"col_3 != 1", initParams{4, 16}, []int{2, 6, 10, 14}, 3, false},

		{"col_0 != 1", initParams{4, 16}, []int{3, 7, 11, 15}, 0, false},
		{"col_1 != 1", initParams{4, 16}, []int{3, 7, 11, 15}, 1, false},
		{"col_2 != 1", initParams{4, 16}, []int{3, 7, 11, 15}, 2, false},
		{"col_3 == 1", initParams{4, 16}, []int{3, 7, 11, 15}, 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := newBitMatrix(tt.initParams.squareSize, tt.initParams.bits)
			for _, flatIdx := range tt.setBits {
				bm.SetFlat(flatIdx)
			}
			if got := bm.ColumnIsOne(tt.column); got != tt.want {
				t.Errorf("ColumnIsOne() = %v, want %v", got, tt.want)
			}
		})
	}
}

type setIdx struct {
	row int
	col int
}

func Test_bitMatrix_Get(t *testing.T) {
	type initParams struct {
		squareSize int
		bits       int
	}

	tests := []struct {
		name       string
		initParams initParams
		setIndices []setIdx
	}{
		{"empty", initParams{8, 64}, nil},
		{"(0,0)", initParams{8, 64}, []setIdx{{0, 0}}},
		{"(7,7)", initParams{8, 64}, []setIdx{{7, 7}}},
		{"(0,0),(7,7)", initParams{8, 64}, []setIdx{{0, 0}, {7, 7}}},
		{"(0,7)", initParams{8, 64}, []setIdx{{0, 0}, {7, 7}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := newBitMatrix(tt.initParams.squareSize, tt.initParams.bits)
			for _, ind := range tt.setIndices {
				bm.Set(ind.row, ind.col)
			}
			for r := 0; r < tt.initParams.squareSize; r++ {
				for c := 0; c < tt.initParams.squareSize; c++ {
					want := contained(r, c, tt.setIndices)
					if got := bm.Get(r, c); got != want {
						t.Errorf("Get() = %v, want %v", got, want)
					}
				}
			}
		})
	}
}

func contained(r int, c int, indices []setIdx) bool {
	for _, idx := range indices {
		if r == idx.row && c == idx.col {
			return true
		}
	}
	return false
}

func Test_bitMatrix_NumOnesInCol(t *testing.T) {
	type initParams struct {
		squareSize int
		bits       int
	}
	tests := []struct {
		name       string
		initParams initParams
		setBits    []int
		column     int
		want       int
	}{
		{"empty", initParams{4, 16}, []int{}, 0, 0},
		{"empty", initParams{4, 16}, []int{}, 1, 0},
		{"empty", initParams{4, 16}, []int{}, 2, 0},
		{"empty", initParams{4, 16}, []int{}, 3, 0},

		{"1", initParams{4, 16}, []int{0}, 0, 1},
		{"2", initParams{4, 16}, []int{0, 4}, 0, 2},
		{"3", initParams{4, 16}, []int{0, 4, 8}, 0, 3},
		{"col == 1", initParams{4, 16}, []int{0, 4, 8, 12}, 0, 4},
		{"1", initParams{4, 16}, []int{0}, 1, 0},
		{"1", initParams{4, 16}, []int{0}, 2, 0},
		{"1", initParams{4, 16}, []int{0}, 3, 0},

		{"1", initParams{4, 16}, []int{3}, 3, 1},
		{"2", initParams{4, 16}, []int{3, 7}, 3, 2},
		{"3", initParams{4, 16}, []int{3, 7, 11}, 3, 3},
		{"4", initParams{4, 16}, []int{3, 7, 11, 15}, 3, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := newBitMatrix(tt.initParams.squareSize, tt.initParams.bits)
			for _, flatIdx := range tt.setBits {
				bm.SetFlat(flatIdx)
			}
			if got := bm.NumOnesInCol(tt.column); got != tt.want {
				t.Errorf("NumOnesInCol() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bitMatrix_NumOnesInRow(t *testing.T) {
	type initParams struct {
		squareSize int
		bits       int
	}
	tests := []struct {
		name       string
		initParams initParams
		setBits    []int
		row        int
		want       int
	}{
		{"empty", initParams{4, 16}, []int{}, 0, 0},
		{"empty", initParams{4, 16}, []int{}, 1, 0},
		{"empty", initParams{4, 16}, []int{}, 2, 0},
		{"empty", initParams{4, 16}, []int{}, 3, 0},

		{"empty", initParams{64, 4096}, []int{}, 0, 0},
		{"empty", initParams{128, 128 * 128}, []int{}, 0, 0},
		{"empty", initParams{256, 256 * 256}, []int{}, 0, 0},
		{"empty", initParams{512, 512 * 512}, []int{}, 0, 0},

		{"1", initParams{4, 16}, []int{0}, 0, 1},
		{"2", initParams{4, 16}, []int{0, 1}, 0, 2},
		{"3", initParams{4, 16}, []int{0, 1, 2}, 0, 3},
		{"4", initParams{4, 16}, []int{0, 1, 2, 3}, 0, 4},

		{"1", initParams{4, 16}, []int{12}, 3, 1},
		{"2", initParams{4, 16}, []int{12, 13}, 3, 2},
		{"3", initParams{4, 16}, []int{12, 13, 14}, 3, 3},
		{"4", initParams{4, 16}, []int{12, 13, 14, 15}, 3, 4},

		{"4", initParams{32, 1024}, []int{0, 1, 2, 3}, 0, 4},
		{"1", initParams{32, 1024}, []int{1023}, 31, 1},
		{"2", initParams{32, 1024}, []int{992, 1023}, 31, 2},
		{"3", initParams{32, 1024}, []int{992, 995, 1023}, 31, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := newBitMatrix(tt.initParams.squareSize, tt.initParams.bits)
			for _, flatIdx := range tt.setBits {
				bm.SetFlat(flatIdx)
			}
			if got := bm.NumOnesInRow(tt.row); got != tt.want {
				t.Errorf("NumOnesInRow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bitMatrix_RowIsOne(t *testing.T) {
	type initParams struct {
		squareSize int
		bits       int
	}

	tests := []struct {
		name       string
		initParams initParams
		setBits    []int
		row        int
		want       bool
	}{
		{"empty", initParams{4, 16}, []int{}, 0, false},
		{"empty", initParams{4, 16}, []int{}, 1, false},
		{"empty", initParams{4, 16}, []int{}, 2, false},
		{"empty", initParams{4, 16}, []int{}, 3, false},

		{"all 1", initParams{4, 16}, []int{0, 1, 2, 3}, 0, true},

		{"empty", initParams{64, 4096}, []int{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := newBitMatrix(tt.initParams.squareSize, tt.initParams.bits)
			for _, flatIdx := range tt.setBits {
				bm.SetFlat(flatIdx)
			}

			if got := bm.RowIsOne(tt.row); got != tt.want {
				t.Errorf("RowIsOne() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bitMatrix_RowRangeIsOne(t *testing.T) {
	type initParams struct {
		squareSize int
		bits       int
	}
	type rowRange struct {
		r     int
		start int
		end   int
	}
	tests := []struct {
		name       string
		initParams initParams
		setBits    []int
		rangeInRow rowRange
		want       bool
	}{
		{"empty", initParams{4, 16}, nil, rowRange{0, 0, 4}, false},
		{"empty", initParams{4, 16}, nil, rowRange{1, 0, 4}, false},
		{"empty", initParams{4, 16}, nil, rowRange{2, 0, 4}, false},
		{"empty", initParams{4, 16}, nil, rowRange{3, 0, 4}, false},

		{"true", initParams{4, 16}, []int{0, 1, 2, 3}, rowRange{0, 0, 4}, true},
		{"true", initParams{4, 16}, []int{0, 1, 2, 3, 4, 5, 6}, rowRange{0, 0, 3}, true},
		{"true", initParams{4, 16}, []int{0, 1, 2, 3, 4, 5, 6}, rowRange{0, 0, 2}, true},
		{"true", initParams{4, 16}, []int{0, 1, 2, 3, 4, 5, 6}, rowRange{0, 0, 1}, true},

		{"true", initParams{4, 16}, []int{0, 1, 2, 3, 4, 5, 6}, rowRange{1, 0, 1}, true},
		{"true", initParams{4, 16}, []int{0, 1, 2, 3, 4, 5, 6}, rowRange{1, 0, 2}, true},

		{"false", initParams{4, 16}, []int{0, 1, 2, 3, 4, 5, 6}, rowRange{1, 0, 3}, false},
		{"false", initParams{4, 16}, []int{0, 1, 2, 3, 4, 5, 6}, rowRange{1, 0, 4}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := newBitMatrix(tt.initParams.squareSize, tt.initParams.bits)
			for _, flatIdx := range tt.setBits {
				bm.SetFlat(flatIdx)
			}
			if got := bm.RowRangeIsOne(tt.rangeInRow.r, tt.rangeInRow.start, tt.rangeInRow.end); got != tt.want {
				t.Errorf("RowRangeIsOne() = %v, want %v", got, tt.want)
			}
		})
	}
}
