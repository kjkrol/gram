package main

import (
	"fmt"
	"github.com/kjkrol/gram/plugins/board/cell"
	"math"
	"slices"
	"strings"
)

// what grows in a town by OpenStreetMap: its parks' cells drawn as Cells, a tree standing
// where it has one, every Row metres along a row of them and Density to the hectare of a park,
// each of a kind of Trees by its share
type greensCfg struct {
	Cells   string      `yaml:"cells"`
	Row     float64     `yaml:"row"`
	Density float64     `yaml:"density"`
	Trees   []treeShare `yaml:"trees"`
}

type treeShare struct {
	Kind  string  `yaml:"kind"`
	Share float64 `yaml:"share"`
}

func (g greensCfg) validate() error {
	if g.Cells == "" && len(g.Trees) == 0 {
		return nil
	}
	if len(g.Trees) == 0 || g.Row <= 0 {
		return fmt.Errorf("greens: want the trees' kinds and how far apart a row's stand")
	}
	for _, t := range g.Trees {
		if t.Kind == "" || t.Share <= 0 {
			return fmt.Errorf("greens: a tree wants its kind and a share")
		}
	}
	return nil
}

// what OpenStreetMap has growing: single trees and rows of them, longitude and latitude, and the
// outlines of the parks
type osmGreens struct {
	trees [][2]float64
	rows  [][][2]float64
	parks [][][2]float64
}

// a tree standing by itself: its kind and where, world units
type tree struct {
	kind string
	at   [2]float64
}

// green lays what grows in the town: the parks' cells and the trees, none on a building's cell
// nor off the land
func (w *chart) green(g *osmGreens) {
	cfg := w.r.Greens
	var total float64
	for _, t := range cfg.Trees {
		total += t.Share
	}
	plant := func(x, y float64) {
		cx, cy := int(math.Floor(x)), int(math.Floor(y))
		if cx < 0 || cy < 0 || cx >= w.cols || cy >= w.rows {
			return
		}
		i := cy*w.cols + cx
		if !w.land[i] || w.soils[i] == w.r.Buildings.Cells || w.wet(w.cell(cx, cy)) {
			return
		}
		at := lattice(int64(x*64), int64(y*64)) * total
		kind := cfg.Trees[len(cfg.Trees)-1].Kind
		for _, t := range cfg.Trees {
			if at < t.Share {
				kind = t.Kind
				break
			}
			at -= t.Share
		}
		w.trees = append(w.trees, tree{kind, [2]float64{x * w.size, y * w.size}})
	}
	for _, ring := range g.parks {
		if cfg.Cells != "" {
			w.fill(shape{ring}, func(i int) {
				if w.land[i] && w.soils[i] != w.r.Buildings.Cells {
					w.soils[i] = cfg.Cells
				}
			})
		}
		if cfg.Density <= 0 {
			continue
		}
		// a tree to each square of the density, shaken within it, where it lies in the park
		var ps [][2]float64
		lo, hi := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
		for _, p := range ring {
			x, y := w.place(p[0], p[1])
			ps = append(ps, [2]float64{x, y})
			lo = [2]float64{min(lo[0], x), min(lo[1], y)}
			hi = [2]float64{max(hi[0], x), max(hi[1], y)}
		}
		side := math.Sqrt(10000/cfg.Density) / w.r.Area.Cell
		for gy := math.Floor(lo[1] / side); gy*side < hi[1]; gy++ {
			for gx := math.Floor(lo[0] / side); gx*side < hi[0]; gx++ {
				x := (gx + lattice(int64(gx), int64(gy))) * side
				y := (gy + lattice(int64(gy)+7919, int64(gx))) * side
				if within(ps, x, y) {
					plant(x, y)
				}
			}
		}
	}
	for _, row := range g.rows {
		left := 0.0
		for i := 1; i < len(row); i++ {
			ax, ay := w.place(row[i-1][0], row[i-1][1])
			bx, by := w.place(row[i][0], row[i][1])
			run := math.Hypot(bx-ax, by-ay) * w.r.Area.Cell
			for ; left <= run+1e-6; left += cfg.Row {
				t := left / max(run, 1e-9)
				plant(ax+(bx-ax)*t, ay+(by-ay)*t)
			}
			left -= run
		}
	}
	for _, p := range g.trees {
		plant(w.place(p[0], p[1]))
	}
}

// within reports whether (x, y) lies inside the ring ps
func within(ps [][2]float64, x, y float64) bool {
	in := false
	for i, a := range ps {
		b := ps[(i+1)%len(ps)]
		if (a[1] <= y) != (b[1] <= y) && x < a[0]+(y-a[1])/(b[1]-a[1])*(b[0]-a[0]) {
			in = !in
		}
	}
	return in
}

// clearing moves the trees off the streets, a roundabout's roadway and the water, and the
// buildings off the tunnels' mouths
func (w *chart) clearing(s *streets) {
	// a trunk a metre off the roadway's edge at least: off every road it stands in, onto a
	// roundabout's island where that is nearer, left out where no room is left it
	lanes := w.lanes(1)
	kept := w.trees[:0]
	for _, t := range w.trees {
		at, ok := w.off(lanes, t.at)
		if !ok {
			s.felled++
			continue
		}
		if at != t.at {
			s.shifted++
		}
		t.at = at
		kept = append(kept, t)
	}
	w.trees = kept
	// no building stands over a tunnel's mouth: its head and the cells it runs under first
	r := w.roads
	var mouths [][2]float64
	for _, cells := range []map[cell.ID]bool{r.heads, r.mouths} {
		for c := range cells {
			x, y := w.xy(c)
			mouths = append(mouths, [2]float64{(float64(x) + 0.5) * w.size,
				(float64(y) + 0.5) * w.size})
		}
	}
	w.buildings = slices.DeleteFunc(w.buildings, func(b built) bool {
		sin, cos := math.Sincos(b.turn * math.Pi / 180)
		return slices.ContainsFunc(mouths, func(m [2]float64) bool {
			dx, dy := m[0]-b.mid[0], m[1]-b.mid[1]
			along, across := dx*cos+dy*sin, dy*cos-dx*sin
			return math.Abs(along) <= (b.size[0]+w.size)/2 &&
				math.Abs(across) <= (b.size[1]+w.size)/2
		})
	})
}

func (w *chart) writeTrees(b *strings.Builder) {
	for _, t := range w.trees {
		fmt.Fprintf(b, "    - {kind: %s, at: [%s, %s]}\n", t.kind, tenth(t.at[0]), tenth(t.at[1]))
	}
}

// treed reports whether a tree stands by itself on the cell at x, y
func (w *chart) treed(x, y int) bool {
	if w.treeCells == nil {
		w.treeCells = map[int]bool{}
		for _, t := range w.trees {
			cx, cy := int(t.at[0]/w.size), int(t.at[1]/w.size)
			if cx >= 0 && cy >= 0 && cx < w.cols && cy < w.rows {
				w.treeCells[cy*w.cols+cx] = true
			}
		}
	}
	return w.treeCells[y*w.cols+x]
}
