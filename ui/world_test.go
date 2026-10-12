package ui

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

type installer struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installer) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installer) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installer) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installer) ECS() *goke.ECS                                  { return c.ecs }
func (c *installer) Hosts(...plugin.Host)                            {}

func TestPin_UnderAnEffectFollowsTheEntitiesItIsOn(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	w.Effects().Define("frozen", effect.Spec{effect.Lasts(time.Second)})
	frozen := w.Effects().Named("frozen")
	kind.Define[float64](w.Kinds(), "unit", kind.Spec{
		comp.Load(func(x float64) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
	})
	unit := kind.Named[float64](w.Kinds(), "unit")
	w.Seed(unit.Entry(100).Named("bob"), unit.Entry(300))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installer{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	label := Label("frozen").Under(frozen)
	s := NewScene("main", Layers(label))
	d := s.Layers()[0].(*drawing)
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ecs := ctx.ecs
	ecs.Setup(append(systems, goke.SystemFn{OnInit: d.Init})...)
	ecs.SetPlan(func(rc goke.RunCtx, dt time.Duration) {
		w.RunPlan(rc, dt)
		w.Clock().Replay(rc, dt)
		rc.Sync()
	})
	tick := func(n int) {
		for range n {
			ecs.Tick(time.Second / 10)
		}
	}

	tick(1)
	w.Carrier().Put(control.Nobody, rule.Put(frozen).On(entity.Named("bob")))
	tick(2)
	d.find()
	spots := d.spots[label]
	if len(spots) != 1 || !spots[0].placed || spots[0].x != 105 || spots[0].y != 100 {
		t.Fatalf("spots %+v, want bob alone, at the top middle of its box (105, 100)", spots)
	}
	tick(12)
	d.find()
	if n := len(d.spots[label]); n != 0 {
		t.Fatalf("%d spots after the effect ended, want none", n)
	}
}

type marked struct{}

func TestPin_WhereTaggedFollowsTheEntitiesCarryingTheTag(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	mark := w.Kinds().DefineTag[marked]("marked")
	kind.Define[float64](w.Kinds(), "unit", kind.Spec{
		comp.Load(func(x float64) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(x float64) tag.Tags[marked] {
			if x < 200 {
				return tag.Tags[marked](0).With(mark)
			}
			return 0
		}),
	})
	unit := kind.Named[float64](w.Kinds(), "unit")
	w.Seed(unit.Entry(100), unit.Entry(300))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installer{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	label := Label("here").Where(Tagged(mark))
	s := NewScene("main", Layers(label))
	d := s.Layers()[0].(*drawing)
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(append(systems, goke.SystemFn{OnInit: d.Init})...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, dt time.Duration) { w.RunPlan(rc, dt); rc.Sync() })
	ctx.ecs.Tick(time.Second / 10)
	d.find()
	spots := d.spots[label]
	if len(spots) != 1 || spots[0].x != 105 {
		t.Fatalf("spots %+v, want the marked unit alone, at x 105", spots)
	}
}

type score struct{ N int8 }

func TestText_OfReadsTheComponentOfTheEntityPinned(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	at := func(x float64) comp.Comp {
		return comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)})
	}
	kind.Define[int8](w.Kinds(), "scored", kind.Spec{
		comp.Load(func(n int8) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(100*float64(n), 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(n int8) score { return score{N: n} }),
	})
	kind.Define[struct{}](w.Kinds(), "plain", kind.Spec{at(900), comp.Const(world.Velocity{})})
	scored, plain := kind.Named[int8](w.Kinds(), "scored"), kind.Named[struct{}](w.Kinds(), "plain")
	w.Seed(scored.Entry(1), scored.Entry(2), plain.Entry(struct{}{}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installer{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	src := Of(func(s *score) (string, bool) { return string(rune('0' + s.N)), true })
	d := NewScene("main", Layers(LabelOf(src).On(nobody))).Layers()[0].(*drawing)
	var base goke.Comp[world.Base]
	var q *goke.Query
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: d.Init}, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&base).Build()
	}})
	ctx.ecs.Setup(systems...)
	said := map[float64]string{}
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i, id := range cur.IDs {
			if text, ok := src.Text(id, true); ok {
				said[base.Slice(cur)[i].Pos.TopLeft.X] = text
			}
		}
	}
	if len(said) != 2 || said[100] != "1" || said[200] != "2" {
		t.Fatalf("said %v, want 1 at x 100 and 2 at x 200, nothing for the one without a score", said)
	}
}
