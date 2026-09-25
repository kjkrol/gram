package main

import (
	"math"
	"math/rand/v2"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
)

// islandLayout draws a fixed island: a wavy ellipse of fields in a sea, mountains ringed by
// hills in the middle rising smoothly from the fields, forests scattered about, and a road looping round the mountains down to
// a southern harbour.
func islandLayout(grid board.Grid) (board.Layout, []board.CellID) {
	rng := rand.New(rand.NewPCG(1, 2))
	cell := func(x, y int) board.CellID { c, _ := grid.CellIndex(uint32(x), uint32(y)); return c }
	kinds := map[board.CellID]string{}

	cx, cy := float64(GridWidth)/2, float64(GridHeight)/2
	inside := func(x, y int) bool {
		dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
		a := math.Atan2(dy, dx)
		r := 1 + 0.12*math.Sin(3*a+1) + 0.08*math.Sin(5*a+2) + 0.05*math.Sin(7*a)
		return (dx*dx)/(islandRX*islandRX)+(dy*dy)/(islandRY*islandRY) <= r*r
	}
	dist := func(x, y int) float64 { return math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) }

	for y := range GridHeight {
		for x := range GridWidth {
			if !inside(x, y) {
				continue
			}
			switch d := dist(x, y); {
			case d <= mountainR:
				kinds[cell(x, y)] = "mountain"
			case d <= hillsR:
				kinds[cell(x, y)] = "hills"
			default:
				kinds[cell(x, y)] = "field"
			}
		}
	}

	for range forestCount {
		fx, fy := rng.IntN(GridWidth), rng.IntN(GridHeight)
		if !inside(fx, fy) || dist(fx, fy) <= hillsR+2 {
			continue
		}
		radius := 4 + rng.Float64()*3
		for y := range GridHeight {
			for x := range GridWidth {
				if math.Hypot(float64(x-fx), float64(y-fy)) <= radius && kinds[cell(x, y)] == "field" {
					kinds[cell(x, y)] = "forest"
				}
			}
		}
	}

	// The road: a hexagon of stops round the hills, joined by L-shaped runs, and a spur south.
	var stops [][2]int
	for k := range 6 {
		a := float64(k) * math.Pi / 3
		stops = append(stops, [2]int{int(cx + roadR*math.Cos(a)), int(cy + roadR*0.75*math.Sin(a))})
	}
	lay := func(from, to [2]int) {
		x, y := from[0], from[1]
		for x != to[0] {
			x += sign(to[0] - x)
			pave(kinds, cell(x, y))
		}
		for y != to[1] {
			y += sign(to[1] - y)
			pave(kinds, cell(x, y))
		}
	}
	pave(kinds, cell(stops[0][0], stops[0][1]))
	for k := range stops {
		lay(stops[k], stops[(k+1)%len(stops)])
	}
	harbour := stops[1]
	for y := harbour[1]; inside(harbour[0], y); y++ {
		pave(kinds, cell(harbour[0], y))
	}

	var cells []board.CellEntry
	for c, k := range kinds {
		cells = append(cells, board.CellEntry{Kind: k, Cell: c})
	}
	var road []board.CellID
	for _, s := range stops {
		road = append(road, cell(s[0], s[1]))
	}
	cw, ch := grid.CellBounds()
	heights := func(p geom.Vec) float64 {
		d := math.Hypot(p.X/cw-cx, p.Y/ch-cy)
		return hillsHeight*rim(d, hillsR) + (mountainHeight-hillsHeight)*rim(d, mountainR)
	}
	return board.Layout{Default: "water", Cells: cells, Heights: heights}, road
}

// rim is 1 inside radius r and 0 outside, easing between the two across one cell.
func rim(d, r float64) float64 {
	t := min(max(r+0.5-d, 0), 1)
	return t * t * (3 - 2*t)
}

// pave lays road on c unless the mountain is in the way — roads go round it.
func pave(kinds map[board.CellID]string, c board.CellID) {
	if kinds[c] != "mountain" {
		kinds[c] = "road"
	}
}

func sign(v int) int {
	if v < 0 {
		return -1
	}
	return 1
}

const (
	islandRX, islandRY = 34.0, 22.0
	mountainR          = 7.0
	hillsR             = 12.0
	hillsHeight        = 20.0
	mountainHeight     = 40.0
	roadR              = 19.0
	forestCount        = 10
)
