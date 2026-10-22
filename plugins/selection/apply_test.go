package selection_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

type installCtx struct {
	hosts   []host.Host
	ecs     *goke.ECS
	pending []func() []goke.System
	pool    *parallel.Pool
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

type unit struct {
	x        float64
	by       command.Issuer
	selected bool
}

func TestSelected_IsThePlayersSelectedUnitsAlone(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	sel := selection.NewPlugin(w)
	w.Write().Effects().Define("haste", effect.Spec{effect.Lasts(time.Hour)})
	haste := w.Write().Effects().Named("haste")
	tags := sel.Tags()
	kind.Define[unit](w.Kinds(), "unit", kind.Spec{
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
	k := kind.Named[unit](w.Kinds(), "unit")
	units := []unit{{x: 100, by: 1, selected: true}, {x: 300, by: 1}, {x: 500, by: 2, selected: true}}
	for _, u := range units {
		w.Write().Seed(k.Entry(u))
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
		w.Write().Clock().Replay(rc, d)
		rc.Sync()
	})
	w.Write().Carrier().Put(1, ceremony.Put(haste).On(sel.Selected()))
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
		if got := w.Read(0).Effects().Has(byX[u.x], haste); got != want {
			t.Errorf("player %d's unit, selected %v: under haste %v, want %v", u.by, u.selected, got, want)
		}
	}
}

func (c *installCtx) Hosts(h ...host.Host)   { c.hosts = append(c.hosts, h...) }
func (c *installCtx) Draws(...render.Source) {}
func (c *installCtx) Looks(func())           {}
func (c *installCtx) Pool() *parallel.Pool   { return c.pool }

func (c *installCtx) Deliver(rules ...act.Rule) error { return ceremony.Deliver(c.hosts, rules...) }
