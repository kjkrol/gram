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
	"github.com/kjkrol/gram/plugins/dialog"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// host stands at home looking west, the way the traveller comes, and talks to him in sight from
// its own script; what it makes of him afterwards is its Regard.
type host struct{ talks *dialog.Plugin }

var (
	_ board.Figure = host{}
	_ board.Ruled  = host{}
)

func (host) Rules(b *board.Plugin) {
	roles, fx := b.Read(0).World().Roles(), b.Read(0).World().Write().Effects()
	talking, talked := fx.Named(dialog.TalkingEf), fx.Named(dialog.TalkedEf)
	roles.Define(HostRole,
		act.Branch[vision.Sighting]{
			Name: "talk",
			When: cond.Other.If(roles.Named(TravellerRole)).
				And(cond.Self.If(cond.Not(talking))).
				And(cond.Self.If(cond.Not(talked))),
			Then: act.Self.Do(dialog.Begin{}),
		},
		act.Branch[vision.Sighting]{
			Name: "part",
			When: cond.Self.If(vision.SeesNobody()),
			Then: act.Self.Take(talking),
		},
	)
}

func (h host) Define(b *board.Plugin) {
	units(b).Define(HostKind, unit.Mover{Domain: cell.Land}, walks,
		ceremony.Plays(b.Read(0).World().Roles().Named(HostRole)),
		comp.Const(vision.Sight{Facing: geom.NewVec(-1, 0), Radius: CellSize * 2.5}),
		comp.Const(world.Eye{Angle: math.Pi / 3}),
		comp.Load(func(u unitRow) dialog.Script { return h.talks.Script(u.Talks) }),
		comp.Const(Regard{}),
	)
}

func (host) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[unitRow](w.Kinds(), HostKind), unitSize, render.Solid(hostColor))
}
