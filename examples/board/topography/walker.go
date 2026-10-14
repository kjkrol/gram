package main

import (
	"github.com/kjkrol/gram/plugins/selection"
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
)

const (
	walkerPx = 22
	sprint   = 4
)

// three quarters of a cell a second: a man walks 1.4 m/s
var walkerSpeed = scale.Units(250)

// walker is a figure on foot: mortal, looking ahead, standing on the ground under gravity; an
// ordered one sets out for its target from the map.
type walker struct {
	kind    string
	color   color.RGBA
	ordered bool
}

var _ board.Figure = walker{}

func (wk walker) Define(b *board.Plugin) {
	units := board.NewUnits[unitRow](b,
		board.Shape{Size: scale.Figure(0.6), Height: figure},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: walkerSpeed,
		Sprint:   sprint,
		Accel:    walkerSpeed * 2,
		Brake:    walkerSpeed * 4,
		V0:       walkerSpeed / 2,
		TurnRate: 0.15,
	}
	extra := append(looking(),
		ceremony.Plays(b.World().Roles().Named(MortalRole)),
		comp.Const(gravity.Body{Leap: scale.Figure(1.5)}))
	if wk.ordered {
		extra = append(extra, ordered())
	}
	units.Define(wk.kind, unit.Mover{Domain: cell.Land}, profile, extra...)
}

func (wk walker) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), wk.kind), walkerPx, render.Solid(wk.color)).
		Styled(selection.Highlight{Line: wk.color, Width: 2})
}

// the eyes of a unit: a sight the way it goes, as wide as a man's
func looking() []comp.Comp {
	return []comp.Comp{
		comp.Const(vision.Sight{
			Facing: geom.NewVec(1, 0),
			Radius: scale.Units(10000),
			Ahead:  true,
		}),
		comp.Const(world.Eye{Angle: 72 * math.Pi / 180}),
	}
}

func ordered() comp.Comp {
	return comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.MoveOrder{Target: u.Target}
	})
}
