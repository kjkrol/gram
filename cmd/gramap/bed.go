package main

import "math"

// bedCfg is the sea's bed: Zoom the Terrain Tiles' zoom its depths are read at, one that has
// them, and Deepest the metres under the sea's level it lies at most. No Zoom: no bed, the sea's
// ground at its level.
type bedCfg struct {
	Zoom    int     `yaml:"zoom"`
	Deepest float64 `yaml:"deepest"`
}

// how little a round of evening out the bed by the coast moves it, metres, for the rounds to
// end, and how many there are at most
const (
	bedSettled = 0.01
	bedRounds  = 4000
)

// sink lays the sea's bed: every corner the sea alone touches as deep as deep tells, whole
// metres, one at least and Deepest at most; where deep tells land under the sea, coarser than
// the map by the coast, the bed runs even from the shore, at the sea's level, to the depths told
func (w *chart) sink(deep elevation) error {
	cols, rows := w.cols+1, w.rows+1
	told := make([]bool, cols*rows)
	var even []int
	for y := range rows {
		for x := range cols {
			if !w.afloat(x, y) {
				told[y*cols+x] = true
				continue
			}
			h, err := deep(w.lonLat(float64(x), float64(y)))
			if err != nil {
				return err
			}
			k := y*cols + x
			if h < 0 {
				w.corners[k], told[k] = -min(-h, w.r.Bed.Deepest), true
			} else {
				even = append(even, k)
			}
		}
	}
	for range bedRounds {
		moved := 0.0
		for _, k := range even {
			x, y := k%cols, k/cols
			sum, n := 0.0, 0
			for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				if nx, ny := x+d[0], y+d[1]; nx >= 0 && ny >= 0 && nx < cols && ny < rows {
					sum += w.corners[ny*cols+nx]
					n++
				}
			}
			h := sum / float64(n)
			moved = max(moved, math.Abs(h-w.corners[k]))
			w.corners[k] = h
		}
		if moved < bedSettled {
			break
		}
	}
	for y := range rows {
		for x := range cols {
			if w.afloat(x, y) {
				w.corners[y*cols+x] = min(math.Round(w.corners[y*cols+x]), -1)
			}
		}
	}
	return nil
}

// afloat reports whether no cell round the corner is land
func (w *chart) afloat(x, y int) bool {
	for _, d := range [4][2]int{{-1, -1}, {0, -1}, {-1, 0}, {0, 0}} {
		cx, cy := x+d[0], y+d[1]
		if cx >= 0 && cy >= 0 && cx < w.cols && cy < w.rows && w.land[cy*w.cols+cx] {
			return false
		}
	}
	return true
}
