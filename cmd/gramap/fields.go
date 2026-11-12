package main

import (
	"cmp"
	"slices"

	"github.com/kjkrol/gram/plugins/board/cell"
)

// a field over a block's cells of a kind: the rectangle round them, corner to corner, and how
// many they are
type field struct {
	kind     string
	from, to cell.ID
	cells    int
}

// fields are the recipe's fields over every block of the board, by the cell they begin at
func (w *chart) fields() []field {
	var out []field
	for _, f := range w.r.Fields {
		for by := 0; by < w.rows; by += f.Grain {
			for bx := 0; bx < w.cols; bx += f.Grain {
				if fd, ok := w.block(f, bx, by); ok {
					out = append(out, fd)
				}
			}
		}
	}
	slices.SortStableFunc(out, func(a, b field) int { return cmp.Compare(a.from, b.from) })
	return out
}

func (w *chart) block(f fieldCfg, bx, by int) (field, bool) {
	x0, y0, x1, y1, n := w.cols, w.rows, -1, -1, 0
	for y := by; y < min(by+f.Grain, w.rows); y++ {
		for x := bx; x < min(bx+f.Grain, w.cols); x++ {
			if slices.Contains(f.Cells, w.kindAt(x, y)) || f.Trees && w.treed(x, y) {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
				n++
			}
		}
	}
	if n < f.Least {
		return field{}, false
	}
	return field{kind: f.Kind, from: w.cell(x0, y0), to: w.cell(x1, y1), cells: n}, true
}

// kindAt is the kind the board's drawing writes at x, y
func (w *chart) kindAt(x, y int) string {
	if !w.land[y*w.cols+x] {
		return w.r.Kinds.Sea
	}
	return w.soils[y*w.cols+x]
}
