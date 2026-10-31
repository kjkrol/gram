package main

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	scoutPx     = 16
	scoutSpeed  = HexSize * 3
	scoutWeight = 80 // kg
)

// scout is a walker looking ahead, sent to its target from the map; one kind a colour.
type scout struct {
	kind  string
	color color.RGBA
}

var _ board.Figure = scout{}

func (s scout) Define(b *board.Plugin) {
	profile := steering.Steering{
		MaxSpeed: scoutSpeed,
		Accel:    scoutSpeed * 2,
		Brake:    scoutSpeed * 4,
		V0:       scoutSpeed / 2,
		TurnRate: 0.15,
	}
	walks := unit.Mover{Domain: cell.Land, Weight: scoutWeight}
	units(b).Define(s.kind, walks, profile, ordered(), sight(), eye(1.5))
}

// each side outlined and routed in its own colour
func (s scout) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), s.kind), scoutPx, render.Hexagon(s.color)).
		Styled(selection.Highlight{Line: s.color, Width: 2}).
		Styled(navigation.RouteStyle{Line: s.color, Goal: s.color, Width: 1.5})
}

func units(b *board.Plugin) *board.Units[unitRow] {
	return board.NewUnits[unitRow](b, board.Shape{Size: scoutPx, Height: 2},
		func(u unitRow) geom.Vec { return b.Read(0).Board().CellCenter(u.Start) })
}

func ordered() comp.Comp {
	return comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.MoveOrder{Target: u.Target}
	})
}

func sight() comp.Comp {
	return comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: 200, Ahead: true})
}

func eye(height float32) comp.Comp {
	return comp.Const(world.Eye{Height: height, Angle: 2 * math.Pi / 5})
}
