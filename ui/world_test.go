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
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// installer is the plugin.Installer a Stage would hand over, minus the engine.
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

// An element Under an effect is shown for the entity the effect is on, at its top, and goes once
// the effect is over.
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
	s := NewScene("main", nil, func() *Element { return Layers(label) })
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
	w.Carrier().Put(control.Nobody, rule.Cast(frozen).On(entity.Named("bob")))
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
