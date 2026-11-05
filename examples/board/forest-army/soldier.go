package main

import (
	"image/color"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	soldierPx = 10
	// a soldier touching a foe strikes now and then, no sooner than a swing after the last; a
	// second blow before the first heals fells
	strikes = 0.2
	swing   = 500 * time.Millisecond
	healing = 8 * time.Second
	// a charge lasts so long past a soldier's last blow, longer than its march at the other army
	engage = 20 * time.Second
)

type soldierRow struct{}

// soldier is a part of an army: it fells the trees it strikes, fights the foes it touches and
// goes as its army folds; engaged, it charges the other army, again and again while it strikes.
type soldier struct {
	kind, role, side, foe, plan string
	// the other army's name
	foeArmy        string
	color, wounded color.RGBA
}

var (
	_ board.Figure = soldier{}
	_ board.Ruled  = soldier{}
)

// a soldier strikes the foes it meets as their boxes touch (collision) and those it is refused
// a step towards for the cell they hold (navigation)
func (s soldier) Rules(b *board.Plugin) {
	w := b.Read(0).World()
	fx := w.Write().Effects()
	swung := fx.Named(BlowEf)
	foe := cond.Other.If(w.Roles().Named(s.foe)).
		And(cond.Self.If(cond.Not(swung))).
		And(cond.Self.If(cond.Chance(strikes)))
	w.Roles().Define(s.role,
		act.Branch[world.Folding]{
			Name: "fall in",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(world.Despawn{}),
		},
		act.Branch[collision.Meeting]{Name: "strike as met", When: foe, Then: strike(w)},
		act.Branch[navigation.Touch]{Name: "strike as touched", When: foe, Then: strike(w)})
}

// strike wounds a mortal foe, or fells one wounded; it keeps an engaged soldier charging
func strike(w *world.Plugin) act.Step {
	fx := w.Write().Effects()
	wounded := fx.Named(WoundedEf)
	engaged := fx.Named(EngagedEf)
	return act.Branch[any]{
		When: cond.Other.If(w.Roles().Named(MortalRole)),
		Then: act.Steps(
			act.Self.Put(fx.Named(BlowEf)),
			act.Branch[any]{When: cond.Self.If(engaged), Then: act.Self.Put(engaged)},
			act.OneOf(
				act.Branch[any]{
					When: cond.Other.If(wounded),
					Then: act.Other.Do(world.Despawn{}),
				},
				act.Other.Put(wounded))),
	}
}

func (s soldier) Define(b *board.Plugin) {
	w := b.Read(0).World()
	// a part: SpawnPart lays its box off the army's centre
	units := board.NewUnits[soldierRow](b, board.Shape{Size: soldierPx},
		func(soldierRow) geom.Vec { return geom.Vec{} })
	units.Define(s.kind, unit.Mover{Domain: cell.Land}, marching,
		charge(w, s.plan, s.foeArmy),
		ceremony.Plays(w.Roles().Named(s.role), w.Roles().Named(s.side),
			w.Roles().Named(FellerRole), w.Roles().Named(MortalRole)))
}

// charge is an engaged soldier's: at foeArmy, again before its soldiers have it fold
func charge(w *world.Plugin, name, foeArmy string) plan.Planned {
	return plan.New(name, plan.OneOf(
		plan.Loop[any]{
			Name: "charge",
			When: cond.Self.If(w.Write().Effects().Named(EngagedEf)),
			Then: plan.Steps(
				plan.Self.Do(navigation.MoveTo{To: ceremony.Named(foeArmy)}),
				plan.Self.Wait(regive)),
		},
		plan.Loop[any]{Name: "rest", When: cond.Self.If(cond.Always), Then: plan.Self.Idle()},
	))
}

func (s soldier) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[soldierRow](w.Kinds(), s.kind), soldierPx, render.Solid(s.color)).
		Under(w.Write().Effects().Named(WoundedEf), render.Solid(s.wounded)).
		Styled(selection.Highlight{Line: s.color, Width: 2}).
		Styled(navigation.RouteStyle{Line: s.color, Goal: s.color, Width: 1}).
		Styled(minimap.Mark{Color: s.color, Size: 3})
}
