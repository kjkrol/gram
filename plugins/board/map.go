package board

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/internal/draw"
	"github.com/kjkrol/gram/plugins/board/look"
)

// Map is what a board is drawn and priced by beyond what its cells say: a look.Map — how the cells
// lie on the screen, what lies over them beyond their sprites, how high they stand — and what a
// step costs beyond its kind's cost (Climb, Least, Slope). The board's own map is the simple map: a
// flat world seen from above, every kind in its Color or drawn sprite, ways and crossings as plain
// bands, a step at its kind's cost times the distance. plugins/topography is the other: a map in
// relief, lit and shaded, seen from above or isometrically.
type Map interface {
	look.Map
	// Climb is how many times as long the step from one cell to its neighbour takes whoever moves
	// in d as it would on the flat: the slope's; 1 on a flat map.
	Climb(from, to cell.ID, d cell.Domain) float64
	// Least is the smallest Climb for d — the planner's estimate counts on it; 1 on a flat map.
	Least(d cell.Domain) float64
	// Slope is how many times as long moving at p towards dir takes whoever moves in d: the slope
	// under the entity; 1 on a flat map.
	Slope(p, dir geom.Vec, d cell.Domain) float64
}

// simpleMap is the board's own Map: flat, from above, plain bands for the ways.
type simpleMap struct {
	look     look.Look
	dressing *draw.Bands
}

// newSimpleMap is the simple map over brd.
func newSimpleMap(brd *Board) *simpleMap {
	return &simpleMap{look: look.FlatLook(), dressing: draw.NewBands(brd)}
}

func (m *simpleMap) Look() look.Look                               { return m.look }
func (*simpleMap) Heights() ground.Heights                         { return nil }
func (m *simpleMap) Dressing() look.Dressing                       { return m.dressing }
func (*simpleMap) Top(cell.ID) (corners [4]float32, level float32) { return corners, 0 }
func (*simpleMap) Climb(cell.ID, cell.ID, cell.Domain) float64     { return 1 }
func (*simpleMap) Least(cell.Domain) float64                       { return 1 }
func (*simpleMap) Slope(geom.Vec, geom.Vec, cell.Domain) float64   { return 1 }

// mapRef is the plugin's Map as it is when asked: what its renderer draws by, WithMap called after
// WithRenderer too.
type mapRef struct{ p *Plugin }

func (m mapRef) Look() look.Look                                   { return m.p.mapping.Look() }
func (m mapRef) Heights() ground.Heights                           { return m.p.mapping.Heights() }
func (m mapRef) Dressing() look.Dressing                           { return m.p.mapping.Dressing() }
func (m mapRef) Top(c cell.ID) (corners [4]float32, level float32) { return m.p.mapping.Top(c) }
