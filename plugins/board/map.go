package board

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/internal/draw"
	"github.com/kjkrol/gram/plugins/board/look"
)

// Map is what a board is drawn and priced by beyond what its cells say, given by WithMap: how
// it looks and what a step costs beyond its kind's cost.
type Map interface {
	// Look is how the cells lie on the screen through a camera.
	Look() look.Look
	// Heights is the height of the ground at any point; nil on a flat map.
	Heights() ground.Heights
	// Dressing lays over the tiles what lies on them beyond their sprites; nil for nothing.
	Dressing() look.Dressing
	// Top is the height of c's corners as drawn (top-left, top-right, bottom-left, bottom-right)
	// with height, its kind's, standing on them, and of its ground; zero on a flat map.
	Top(c cell.ID, height float64) (corners [4]float32, level float32)
	// Climb is how many times as long the step from one cell to its neighbour takes whoever moves
	// in d as it would on the flat; 1 on a flat map.
	Climb(from, to cell.ID, d cell.Domain) float64
	// Least is the smallest Climb for d; 1 on a flat map.
	Least(d cell.Domain) float64
	// Slope is how many times as long moving at p towards dir takes whoever moves in d; 1 on a
	// flat map.
	Slope(p, dir geom.Vec, d cell.Domain) float64
}

// MapRead is a board's Map as a BoardRead reads it: a look.Map, a cell's kind read by the
// BoardRead, and what a step costs.
type MapRead interface {
	look.Map
	Climb(from, to cell.ID, d cell.Domain) float64
	Least(d cell.Domain) float64
	Slope(p, dir geom.Vec, d cell.Domain) float64
}

// readMap is the board's Map as it is when asked, so WithMap after the atlas closed still counts
type readMap struct{ r *BoardRead }

func (m readMap) Look() look.Look         { return m.r.b.mapping.Look() }
func (m readMap) Heights() ground.Heights { return m.r.b.mapping.Heights() }
func (m readMap) Dressing() look.Dressing { return m.r.b.mapping.Dressing() }

func (m readMap) Top(c cell.ID) (corners [4]float32, level float32) {
	return m.r.b.mapping.Top(c, m.r.Kind(c).Height)
}

func (m readMap) Climb(from, to cell.ID, d cell.Domain) float64 {
	return m.r.b.mapping.Climb(from, to, d)
}

func (m readMap) Least(d cell.Domain) float64 { return m.r.b.mapping.Least(d) }

func (m readMap) Slope(p, dir geom.Vec, d cell.Domain) float64 {
	return m.r.b.mapping.Slope(p, dir, d)
}

type simpleMap struct {
	look     look.Look
	dressing *draw.Bands
}

func newSimpleMap(r *BoardRead) *simpleMap {
	return &simpleMap{look: look.FlatLook(), dressing: draw.NewBands(r)}
}

func (m *simpleMap) Look() look.Look                                        { return m.look }
func (*simpleMap) Heights() ground.Heights                                  { return nil }
func (m *simpleMap) Dressing() look.Dressing                                { return m.dressing }
func (*simpleMap) Top(cell.ID, float64) (corners [4]float32, level float32) { return corners, 0 }
func (*simpleMap) Climb(cell.ID, cell.ID, cell.Domain) float64              { return 1 }
func (*simpleMap) Least(cell.Domain) float64                                { return 1 }
func (*simpleMap) Slope(geom.Vec, geom.Vec, cell.Domain) float64            { return 1 }
