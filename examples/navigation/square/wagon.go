package main

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
)

const (
	wagonPx     = 28 // wider than the bridge's lane between its railings
	wagonSpeed  = CellSize * 2
	wagonWeight = 1500 // kg: more than the rope bridge bears
)

// wagon is too wide for the bridge and too heavy for the rope bridge: its routes go round by the
// ford.
type wagon struct{}

var _ board.Figure = wagon{}

func (wagon) Define(b *board.Plugin) {
	profile := steering.Steering{
		MaxSpeed: wagonSpeed,
		Accel:    wagonSpeed * 2,
		Brake:    wagonSpeed * 4,
		V0:       wagonSpeed / 2,
		TurnRate: 0.1,
	}
	units := board.NewUnits[unitRow](b, board.Shape{Size: wagonPx, Height: wagonTall * wagonPx},
		func(u unitRow) geom.Vec { return b.Read(0).Board().CellCenter(u.Start) })
	units.Define(WagonKind, unit.Mover{Domain: cell.Land, Weight: wagonWeight}, profile,
		ordered())
}

func (wagon) Look(a *world.Atlas, w *world.Plugin) {
	a.AddBody(kind.Named[unitRow](w.Kinds(), WagonKind), wagonPx, wagonBody()).
		Styled(selection.Highlight{Line: wagonColor, Width: 2}).
		Styled(navigation.RouteStyle{Line: wagonColor, Goal: wagonColor, Width: 1.5})
}

// how tall the wagon stands, of how long it is
const wagonTall = 0.58

// wagonBody is a bed on four wheels under the arch of a canvas cover, its pole ahead, running
// along x, in shares of its length
func wagonBody() world.Body {
	at := func(x, y, z float32) [3]float32 {
		return [3]float32{x * wagonPx, y * wagonPx, z * wagonPx}
	}
	parts := []world.Part{
		{Shape: world.Box, From: at(-0.43, 0, 0.22), To: at(0.43, 0, 0.22), Wide: 0.54 * wagonPx,
			Tip: 0.18 * wagonPx},
		{Shape: world.Cone, From: at(-0.36, 0, 0.31), To: at(0.36, 0, 0.31),
			Wide: 0.46 * wagonPx, Tip: 0.46 * wagonPx, Paint: 1},
		{Shape: world.Box, From: at(0.43, 0, 0.18), To: at(0.49, 0, 0.16), Wide: 0.04 * wagonPx,
			Tip: 0.04 * wagonPx},
	}
	for _, x := range []float32{-0.29, 0.29} {
		for _, y := range []float32{-1, 1} {
			parts = append(parts, world.Part{Shape: world.Cone, From: at(x, 0.27*y, 0.14),
				To: at(x, 0.36*y, 0.14), Wide: 0.28 * wagonPx, Tip: 0.28 * wagonPx, Paint: 2})
		}
	}
	return world.Body{
		Wide:  wagonPx,
		Tall:  wagonTall * wagonPx,
		Parts: parts,
		Paints: []world.Paint{
			{Colour: wagonColor, Dots: wagonGrainColor},
			{Colour: canvasColor},
			{Colour: wheelColor},
		},
	}
}
