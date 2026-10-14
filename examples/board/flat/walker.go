package main

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
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
	walkerPx    = 20
	walkerSpeed = CellSize * 3
)

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

// walker is a figure on foot, looking ahead, drowning in the water, sent to its target from
// the map.
type walker struct{}

var (
	_ board.Figure = walker{}
	_ board.Ruled  = walker{}
)

func (walker) Rules(b *board.Plugin) {
	b.World().Roles().Define(MortalRole, act.Branch[unit.Standing]{
		Name: "drown",
		When: cond.Self.If(unit.Fallen()),
		Then: act.Self.Do(world.Despawn{}),
	})
}

func (walker) Define(b *board.Plugin) {
	units := board.NewUnits[unitRow](b, board.Shape{Size: walkerPx},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: walkerSpeed,
		Accel:    walkerSpeed * 2,
		Brake:    walkerSpeed * 4,
		V0:       walkerSpeed / 2,
		TurnRate: 0.15,
	}
	order := comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.MoveOrder{Target: u.Target}
	})
	units.Define(UnitKind, unit.Mover{Domain: cell.Land}, profile,
		order,
		comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: 7 * CellSize, Ahead: true}),
		comp.Const(world.Eye{Angle: 2 * math.Pi / 5}),
		comp.Const(vision.SightOutline{}),
		ceremony.Plays(b.World().Roles().Named(MortalRole)))
}

func (walker) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), UnitKind), walkerPx, render.Solid(unitColor))
}
