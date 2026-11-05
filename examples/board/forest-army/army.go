package main

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
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
	armyPx = 28
	// the soldiers of an army whole, stood in a ring round it as it unfolds
	soldiers = 12
	ringR    = 40
	// a battle on the abstract graph takes a soldier off each side a blow
	blow = time.Second
	// how often a charge gives its order anew: shorter than the fold's wait, so the army it is
	// aimed at stays unfolded while the charge lasts
	regive = 1500 * time.Millisecond
)

var ring = func() (at [soldiers]geom.Vec) {
	for i := range at {
		a := 2 * math.Pi * float64(i) / soldiers
		at[i] = geom.NewVec(float32(ringR*math.Cos(a)), float32(ringR*math.Sin(a)))
	}
	return at
}()

type armyRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

// army is a side's soldiers seen from afar, one point on the abstract graph: come down to L3 it
// stands and musters the soldiers its losses leave, each taking its march over; folded, it takes
// the march back and the losses from those who fell in. Two meeting on the abstract graph stop
// and fight blow by blow as long as the other has soldiers; one with none left goes. Charging, it
// goes at the other army, and musters soldiers who charge, as it does fighting.
type army struct {
	kind, role, plan, side, foe, soldier string
	// the other army's name, and the group its soldiers muster in
	foeArmy, soldiers string
	color             color.RGBA
	// what the soldiers it musters are told as they are made: whose they are
	enlisted func() []command.Command
}

var (
	_ board.Figure = army{}
	_ board.Ruled  = army{}
)

func (r army) Rules(b *board.Plugin) {
	w := b.Read(0).World()
	fx := w.Write().Effects()
	foe := w.Roles().Named(r.foe)
	charging := fx.Named(ChargingEf)
	muster := []act.Step{
		act.Self.Do(selection.Forbid{}),
		act.Self.Take(charging),
	}
	for n := 1; n <= soldiers; n++ {
		muster = append(muster, act.Self.Take(fx.Named(fightEf(n))))
	}
	rules := []act.Rule{
		// its soldiers take the charge or the fight over, ordered by none of its own
		act.Branch[world.Unfold]{
			Name: "charge",
			When: cond.Self.If(charging).Or(fighting(fx)),
			Then: act.Steps(
				act.Self.Do(navigation.Stop{}),
				act.Self.Do(ceremony.Put(fx.Named(EngagedEf)).On(ceremony.Group(r.soldiers)))),
		},
		act.Branch[world.Unfold]{
			Name: "muster",
			When: cond.Self.If(cond.Always),
			Then: act.Steps(muster...),
		},
		act.Branch[world.Fold]{
			Name: "fall in",
			When: cond.Self.If(cond.Always),
			Then: act.Steps(
				act.Self.Do(navigation.Inherit{}),
				act.Self.Do(selection.Allow{})),
		},
		act.Branch[world.Fold]{
			Name: "wiped out",
			When: cond.Self.If(cond.Not(world.Parts(1))),
			Then: act.Self.Do(world.Despawn{}),
		},
		act.Branch[world.Met]{
			Name: "halt",
			When: cond.Other.If(foe),
			Then: act.Self.Do(navigation.Stop{}),
		},
	}
	recruit := kind.EntryOf(r.soldier, soldierRow{}).InGroup(r.soldiers).
		Told(navigation.Inherit{}).Told(r.enlisted()...)
	for left := 1; left <= soldiers; left++ {
		var spawn []act.Step
		for i := range left {
			spawn = append(spawn, act.Self.Do(world.SpawnPart{Entry: recruit, At: ring[i]}))
		}
		lost := []act.Step{}
		for k := 1; k < soldiers; k++ {
			lost = append(lost, act.Self.Take(fx.Named(lossEf(k))))
		}
		if left < soldiers {
			lost = append(lost, act.Self.Put(fx.Named(lossEf(soldiers-left))))
		}
		fell := cond.Self.If(world.Parts(left))
		if left < soldiers {
			fell = fell.And(cond.Self.If(cond.Not(world.Parts(left + 1))))
		}
		rules = append(rules,
			act.Branch[world.Unfold]{
				Name: fmt.Sprintf("unfold %d", left),
				When: strength(cond.Self, fx, left),
				Then: act.Steps(spawn...),
			},
			act.Branch[world.Fold]{
				Name: fmt.Sprintf("fold %d", left),
				When: fell,
				Then: act.Steps(lost...),
			},
			act.Branch[world.Met]{
				Name: fmt.Sprintf("engage %d", left),
				When: cond.Other.If(foe).And(strength(cond.Other, fx, left)),
				Then: act.Self.Put(fx.Named(fightEf(left))),
			})
	}
	w.Roles().Define(r.role, rules...)
}

// strength holds while who has left soldiers: short of none for a whole army
func strength(who cond.Subject, fx *cgoke.Effects, left int) cond.Cond {
	if left < soldiers {
		return who.If(fx.Named(lossEf(soldiers - left)))
	}
	whole := who.If(cond.Not(fx.Named(lossEf(1))))
	for k := 2; k < soldiers; k++ {
		whole = whole.And(who.If(cond.Not(fx.Named(lossEf(k)))))
	}
	return whole
}

func (r army) Define(b *board.Plugin) {
	w := b.Read(0).World()
	units := board.NewUnits[armyRow](b, board.Shape{Size: armyPx},
		func(row armyRow) geom.Vec { return b.Read(0).Board().CellCenter(row.Start) })
	units.Define(r.kind, unit.Mover{Domain: cell.Land}, marching,
		comp.Load(func(row armyRow) navigation.MoveOrder {
			return navigation.MoveOrder{Target: row.Target}
		}),
		comp.Const(world.Aggregate{}),
		// its soldiers stand round it, pushed by nothing of its own
		comp.Without[collision.Collider](),
		comp.Without[collision.Physics](),
		battle(w, r.plan, r.foeArmy),
		ceremony.Plays(w.Roles().Named(r.role), w.Roles().Named(r.side)))
}

// fighting holds while an army has blows of a fight left
func fighting(fx *cgoke.Effects) cond.Cond {
	c := cond.Self.If(fx.Named(fightEf(1)))
	for n := 2; n <= soldiers; n++ {
		c = c.Or(cond.Self.If(fx.Named(fightEf(n))))
	}
	return c
}

// battle is a folded army's fight: a soldier off at every blow, the army with its last, the
// blows counted down; and its charge, at foeArmy wherever it goes
func battle(w *world.Plugin, name, foeArmy string) plan.Planned {
	fx := w.Write().Effects()
	lose := []act.Step{act.Branch[any]{
		When: cond.Self.If(fx.Named(lossEf(soldiers - 1))),
		Then: act.Self.Do(world.Despawn{}),
	}}
	for k := soldiers - 2; k >= 1; k-- {
		lose = append(lose, act.Branch[any]{
			When: cond.Self.If(fx.Named(lossEf(k))),
			Then: swap(fx, lossEf(k), lossEf(k+1)),
		})
	}
	lose = append(lose, act.Self.Put(fx.Named(lossEf(1))))
	count := []act.Step{act.Branch[any]{
		When: cond.Self.If(fx.Named(fightEf(1))),
		Then: act.Self.Take(fx.Named(fightEf(1))),
	}}
	for n := 2; n <= soldiers; n++ {
		count = append(count, act.Branch[any]{
			When: cond.Self.If(fx.Named(fightEf(n))),
			Then: swap(fx, fightEf(n), fightEf(n-1)),
		})
	}
	return plan.New(name, plan.OneOf(
		plan.Loop[any]{
			Name: "fight",
			When: fighting(fx),
			Then: plan.Steps(plan.Self.Wait(blow), act.OneOf(lose...), act.OneOf(count...)),
		},
		plan.Loop[any]{
			Name: "charge",
			When: cond.Self.If(fx.Named(ChargingEf)),
			Then: plan.Steps(
				plan.Self.Do(navigation.MoveTo{To: ceremony.Named(foeArmy)}),
				plan.Self.Wait(regive)),
		},
		plan.Loop[any]{Name: "rest", When: cond.Self.If(cond.Always), Then: plan.Self.Idle()},
	))
}

func swap(fx *cgoke.Effects, from, to string) act.Step {
	return act.Steps(act.Self.Take(fx.Named(from)), act.Self.Put(fx.Named(to)))
}

// unseen while its soldiers stand, a banner of its colour from L4
func (r army) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[armyRow](w.Kinds(), r.kind), armyPx, unseen).
		At(lod.L4, banner(r.color)).
		Styled(selection.Highlight{Line: r.color, Width: 2}).
		Styled(navigation.RouteStyle{Line: r.color, Goal: r.color, Width: 1.5}).
		Styled(minimap.Mark{Color: r.color, Size: 6})
}

func banner(c color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		half := float32(size) / 2
		dst.FillCircle(half, half, half, rimColor)
		dst.FillCircle(half, half, half*0.7, c)
	}
}
