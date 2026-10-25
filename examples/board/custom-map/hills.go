package main

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/render"
)

// hills is the game's own map, given the board by WithMap: flat to the eye from above, its ground
// rising in round hills, a step uphill dearer the steeper it climbs, and every grass tile shaded
// by how high it lies
type hills struct {
	grid   grid.Grid
	board  *board.Plugin
	shades []render.SpriteID // the grass's, low to high, once its kinds are defined
}

var (
	_ board.Map     = (*hills)(nil)
	_ look.Dressing = (*hills)(nil)
)

const (
	climbCost  = 4  // how many times dearer a step is for every cell's height it climbs
	bandHeight = 25 // how high a band of the grass's shades is
)

// a hill: its top at (x, y), r round, h high
type peak struct{ x, y, r, h float32 }

var peaks = []peak{
	{10 * CellSize, 8 * CellSize, 6 * CellSize, 90},
	{22 * CellSize, 12 * CellSize, 7 * CellSize, 120},
	{26 * CellSize, 4 * CellSize, 4 * CellSize, 60},
}

// height is the ground's at p: every hill's, smoothly down to nothing at its foot
func height(p geom.Vec) float32 {
	z := float32(0)
	for _, k := range peaks {
		if d := float32(math.Hypot(float64(p.X-k.x), float64(p.Y-k.y))) / k.r; d < 1 {
			z += k.h * (1 - d*d) * (1 - d*d)
		}
	}
	return z
}

func (*hills) Look() look.Look           { return look.Flat }
func (*hills) Heights() ground.Heights   { return heights{} }
func (m *hills) Dressing() look.Dressing { return m }

func (m *hills) Top(c cell.ID, h float32) (corners [4]float32, level float32) {
	at, half := m.grid.CellCenter(c), float32(CellSize)/2
	for i, d := range [4]geom.Vec{{X: -half, Y: -half}, {X: half, Y: -half}, {X: -half, Y: half},
		{X: half, Y: half}} {
		corners[i] = height(at.Add(d)) + h
	}
	return corners, height(at)
}

func (m *hills) Climb(from, to cell.ID, _ cell.Domain) float64 {
	rise := float64(height(m.grid.CellCenter(to)) - height(m.grid.CellCenter(from)))
	return 1 + climbCost*max(rise, 0)/CellSize
}

func (*hills) Least(cell.Domain) float64 { return 1 }

func (*hills) Slope(p, dir geom.Vec, _ cell.Domain) float64 {
	const run = CellSize / 4
	rise := float64(height(geom.NewVec(p.X+dir.X*run, p.Y+dir.Y*run)) - height(p))
	return 1 + climbCost*max(rise, 0)/run
}

func (*hills) Begin(*render.Frame, camera.Camera) {}

// Base shades a grass tile by the band of height its middle lies in
func (m *hills) Base(c cell.ID, own render.SpriteID) render.SpriteID {
	if m.shades == nil {
		kinds := m.board.Read(0).CellKinds()
		for _, name := range grassShades {
			m.shades = append(m.shades, kinds.Named(name).SpriteID())
		}
	}
	if own != m.shades[0] {
		return own
	}
	band := int(height(m.grid.CellCenter(c)) / bandHeight)
	return m.shades[min(band, len(m.shades)-1)]
}

// heights are the hills' ground for whatever reads it: sight, routes, the weather
type heights struct{}

func (heights) At(p geom.Vec) float32 { return height(p) }
func (heights) Step() float32         { return CellSize / 4 }
