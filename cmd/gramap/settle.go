package main

import (
	"cmp"
	"maps"
	"slices"

	"github.com/kjkrol/gram/plugins/board/cell"
)

// settle lays the settlements' cells round every place, as many as its size: grown from the place
// over dry land no road, rock, bridge nor other settlement takes, a cell beside a road before any
// other, the nearest first
func (w *chart) settle() {
	s := w.r.Settlements
	w.settled = map[string][]cell.ID{}
	taken := map[cell.ID]bool{}
	for _, name := range slices.Sorted(maps.Keys(w.r.Roads.Places)) {
		n, _ := s.cells(w.r.Roads.Places[name].Size)
		if n == 0 {
			continue
		}
		at, ok := w.roads.places[name]
		if !ok {
			w.unsettled = append(w.unsettled, name)
			continue
		}
		cells := w.grow(at, n, taken)
		for _, c := range cells {
			x, y := w.xy(c)
			w.soils[y*w.cols+x] = s.Cells
			taken[c] = true
		}
		w.settled[name] = cells
	}
}

// grow is up to n cells a settlement takes round at, in the order taken
func (w *chart) grow(at cell.ID, n int, taken map[cell.ID]bool) []cell.ID {
	type candidate struct {
		c     cell.ID
		off   bool // off every road
		reach int  // cells squared
	}
	ax, ay := w.xy(at)
	of := func(c cell.ID) candidate {
		x, y := w.xy(c)
		return candidate{c: c, off: !w.byRoad(c), reach: (x-ax)*(x-ax) + (y-ay)*(y-ay)}
	}
	first := func(a, b candidate) bool {
		if a.off != b.off {
			return b.off
		}
		return cmp.Or(cmp.Compare(a.reach, b.reach), cmp.Compare(a.c, b.c)) < 0
	}
	free := func(c cell.ID) bool { return !taken[c] && w.settles(c) }
	seen := map[cell.ID]bool{at: true}
	front := []candidate{of(at)}
	var out []cell.ID
	for len(front) > 0 && len(out) < n {
		i := 0
		for j := range front {
			if first(front[j], front[i]) {
				i = j
			}
		}
		cd := front[i]
		front = slices.Delete(front, i, i+1)
		if cd.c != at || free(at) {
			out = append(out, cd.c)
		}
		for d := range w.grid.Directions() {
			if m, ok := w.grid.Toward(cd.c, d); ok && !seen[m] && free(m) {
				seen[m] = true
				front = append(front, of(m))
			}
		}
	}
	return out
}

// settles reports whether a settlement may take c: dry land no road, rock nor bridge holds
func (w *chart) settles(c cell.ID) bool {
	x, y := w.xy(c)
	if !w.land[y*w.cols+x] || w.wet(c) || w.soils[y*w.cols+x] == w.r.Kinds.Rock {
		return false
	}
	if _, road := w.roads.net.Node(c); road {
		return false
	}
	_, bridged := w.roads.spanOf[c]
	return !bridged
}

func (w *chart) byRoad(c cell.ID) bool {
	for d := range w.grid.Directions() {
		if m, ok := w.grid.Toward(c, d); ok {
			if _, road := w.roads.net.Node(m); road {
				return true
			}
		}
	}
	return false
}
