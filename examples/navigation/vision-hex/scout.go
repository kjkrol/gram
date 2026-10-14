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
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
)

const (
	scoutPx    = 22
	scoutSpeed = HexSize * 3
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
	units(b).Define(s.kind, unit.Mover{Domain: cell.Land}, profile, ordered(), sight(), eye(1.5))
}

func (s scout) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), s.kind), scoutPx, render.Hexagon(s.color))
}

func units(b *board.Plugin) *board.Units[unitRow] {
	return board.NewUnits[unitRow](b, board.Shape{Size: scoutPx, Height: 2},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
}

func ordered() comp.Comp {
	return comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.MoveOrder{Target: u.Target}
	})
}

func sight() comp.Comp {
	return comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: 200, Ahead: true})
}

func eye(height float64) comp.Comp {
	return comp.Const(world.Eye{Height: height, Angle: 2 * math.Pi / 5})
}
