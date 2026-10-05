package selection_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// installCtx is the plugin.Installer a Stage would hand over, minus the engine.
type installCtx struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installCtx) ECS() *goke.ECS                                  { return c.ecs }

// unit is a unit of a test of the selected: whose it is and whether it is selected.
type unit struct {
	x        float64
	by       control.PlayerID
	selected bool
}

// A command for the selected puts its effect on the units the player who gives it owns and has
// selected alone: not its unselected ones, not another player's selected ones.
func TestSelected_IsThePlayersSelectedUnitsAlone(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	sel := selection.NewPlugin(w)
	haste := w.Effects().Define("haste", effect.Spec{effect.Lasts(time.Hour)})
	tags := sel.Tags()
	k := kind.Define[unit](w.Kinds(), "unit", kind.Spec{
		comp.Load(func(u unit) world.Position { return world.Position{AABB: plane.NewAABB(geom.NewVec(u.x, 100), 10, 10)} }),
		comp.Const(world.Velocity{}),
		comp.Load(func(u unit) tag.Tags[selection.Family] {
			t := tag.Tags[selection.Family](0).With(tags.Selectable)
			if u.selected {
				t = t.With(tags.Selected)
			}
			return t
		}),
		comp.Load(func(u unit) tag.Tags[owner.Family] { return tag.Tags[owner.Family](0).With(owner.Of(u.by)) }),
	})
	units := []unit{{x: 100, by: 1, selected: true}, {x: 300, by: 1}, {x: 500, by: 2, selected: true}}
	for _, u := range units {
		w.Seed(k.Entry(u))
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := sel.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := w.Carry(sel); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	var base goke.Comp[world.Base]
	var q *goke.Query
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base).Build() }})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		sel.RunPlan(rc, d)
		w.RunPlan(rc, d)
		w.Clock().Replay(rc, d)
		rc.Sync()
	})
	w.Commands().Put(1, rule.Cast(haste).On(sel.Selected()))
	for range 2 {
		ctx.ecs.Tick(time.Second / 10)
	}
	byX := map[float64]uid.UID64{}
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i, id := range cur.IDs {
			byX[base.Slice(cur)[i].Pos.TopLeft.X] = id
		}
	}
	for _, u := range units {
		want := u.by == 1 && u.selected
		if got := haste.On(byX[u.x]); got != want {
			t.Errorf("player %d's unit, selected %v: under haste %v, want %v", u.by, u.selected, got, want)
		}
	}
}
