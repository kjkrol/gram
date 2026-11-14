package main

import (
	"image/color"
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
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	walkerPx = 22
	sprint   = 4
)

// five metres a second, where a man walks 1.4: the island is crossed in a game's time
var walkerSpeed = scale.Figure(5)

// walking is how whoever goes on foot steers, a caravan's wagon keeping its porters' pace
var walking = steering.Steering{
	MaxSpeed: walkerSpeed,
	Sprint:   sprint,
	Accel:    walkerSpeed * 2,
	Brake:    walkerSpeed * 4,
	V0:       walkerSpeed / 2,
	TurnRate: 0.15,
}

// walker is a figure on foot: mortal, looking ahead, standing on the ground under gravity; an
// ordered one sets out for its target from the map, a highland one climbs the steeper slopes, a
// member one is a part of an aggregate.
type walker struct {
	kind     string
	color    color.RGBA
	ordered  bool
	highland bool
	// the role it plays as a part, laid off its whole's centre and folding in with it; none for
	// a walker of its own
	member string
	// its banner far off; none where zero
	banner world.BannerStyle
}

var (
	_ board.Figure = walker{}
	_ board.Ruled  = walker{}
)

func (wk walker) Rules(b *board.Plugin) {
	if wk.member == "" {
		return
	}
	b.Read(0).World().Roles().Define(wk.member, act.Branch[world.Folding]{
		Name: "fold in",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(world.Despawn{}),
	})
}

func (wk walker) Define(b *board.Plugin) {
	roles := b.Read(0).World().Roles()
	at := func(u unitRow) geom.Vec { return b.Read(0).Board().CellCenter(u.Start) }
	plays := []*ceremony.Part{roles.Named(MortalRole)}
	if wk.member != "" {
		at = func(unitRow) geom.Vec { return geom.Vec{} }
		plays = append(plays, roles.Named(wk.member))
	}
	units := board.NewUnits[unitRow](b, board.Shape{Size: scale.Figure(0.6), Height: figure}, at)
	extra := append(looking(), ceremony.Plays(plays...), leaping())
	if wk.ordered {
		extra = append(extra, ordered())
	}
	domain := cell.Land
	if wk.highland {
		domain |= HighlandDomain
	}
	units.Define(wk.kind, unit.Mover{Domain: domain}, walking, extra...)
}

// a figure on foot under gravity leaps a little
func leaping() comp.Comp { return comp.Const(gravity.Body{Leap: scale.Figure(1.5)}) }

func (wk walker) Look(a *world.Atlas, w *world.Plugin) {
	slot := a.Add(kind.Named[unitRow](w.Kinds(), wk.kind), walkerPx, render.Solid(wk.color)).
		Upright().
		Styled(selection.Highlight{Line: wk.color, Width: 2}).
		Styled(minimap.Mark{Color: wk.color, Size: 5})
	if wk.banner != (world.BannerStyle{}) {
		slot.Styled(wk.banner)
	}
}

// the eyes of a unit: a sight the way it goes, as wide as a man's
func looking() []comp.Comp {
	return []comp.Comp{
		comp.Const(vision.Sight{
			Facing: geom.NewVec(1, 0),
			Radius: scale.Units(here.sees),
			Ahead:  true,
		}),
		comp.Const(world.Eye{Angle: 72 * math.Pi / 180, Vertical: 50 * math.Pi / 180}),
	}
}

func ordered() comp.Comp {
	return comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.MoveOrder{Target: u.Target}
	})
}
