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
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// camp is four units of one kind that carries nothing of the selection or of the players: what
// they are — whose, selectable — they are told as they are made.
type camp struct {
	t       *testing.T
	w       *world.Plugin
	sel     *selection.Plugin
	players *players.Plugin
	one     *players.Player
	two     *players.Player
	haste   effect.Effect
	ecs     *goke.ECS
	base    goke.Comp[world.Base]
	q       *goke.Query
}

func newCamp(t *testing.T, entries func(c *camp, unit kind.Of[float64]) []kind.Entry) *camp {
	t.Helper()
	c := &camp{t: t}
	c.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 10, MaxSize: 10},
	})
	kind.Define[float64](c.w.Kinds(), "unit", kind.Spec{
		comp.Load(func(x float64) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
	})
	unit := kind.Named[float64](c.w.Kinds(), "unit")
	c.sel = selection.NewPlugin(c.w)
	c.players = players.NewPlugin(c.w, c.sel)
	c.one, c.two = c.players.Local("one"), c.players.Add("two")
	c.w.Effects().Define("haste", effect.Spec{effect.Lasts(time.Hour)})
	c.haste = c.w.Effects().Named("haste")
	c.w.Seed(entries(c, unit)...)
	if err := c.w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{c.w, c.sel, c.players} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) { c.q = si.NewQueryBuilder(&c.base).Build() }})
	c.ecs = ctx.ecs
	c.ecs.Setup(systems...)
	c.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		c.sel.RunPlan(rc, d)
		c.w.RunPlan(rc, d)
		c.w.Clock().Replay(rc, d)
		rc.Sync()
		c.players.RunPlan(rc, d)
	})
	c.tick()
	return c
}

func (c *camp) tick() {
	for range 2 {
		c.ecs.Tick(time.Second / 10)
	}
}

// ids are the units by where they stand.
func (c *camp) ids() map[float64]uid.UID64 {
	out := map[float64]uid.UID64{}
	for c.q.All(); c.q.Next(); {
		cur := c.q.Cursor()
		for i, id := range cur.IDs {
			out[c.base.Slice(cur)[i].Pos.TopLeft.X] = id
		}
	}
	return out
}

// reached are the units, by where they stand, that by reaches: it selects everything and casts
// haste on what it has selected; the haste is lifted again after.
func (c *camp) reached(by control.PlayerID) map[float64]bool {
	c.t.Helper()
	everywhere := geom.NewAABBAt(geom.NewVec(0, 0), 1000, 1000)
	c.w.Commands().Put(by, selection.Select{Box: everywhere})
	c.tick()
	c.w.Commands().Put(by, rule.Cast(c.haste).On(c.sel.Selected()))
	c.tick()
	out := map[float64]bool{}
	for x, id := range c.ids() {
		if c.haste.On(id) {
			out[x] = true
		}
	}
	c.w.Commands().Put(by, rule.Lift(c.haste).On(c.sel.Selected()))
	c.tick()
	c.w.Commands().Put(by, selection.Select{IDs: []uid.UID64{}})
	c.tick()
	return out
}

func only(got map[float64]bool, want ...float64) bool {
	if len(got) != len(want) {
		return false
	}
	for _, x := range want {
		if !got[x] {
			return false
		}
	}
	return true
}

// A unit is whose its entry tells it and selectable where told so: each player reaches its own,
// nobody the ownerless one, and no one the unit never allowed.
func TestTold_AUnitIsWhoseAndSelectableAsItsEntrySays(t *testing.T) {
	c := newCamp(t, func(c *camp, unit kind.Of[float64]) []kind.Entry {
		return []kind.Entry{
			unit.Entry(100).Told(players.Give{To: c.one.ID}, selection.Allow{}),
			unit.Entry(200).Told(players.Give{To: c.two.ID}, selection.Allow{}),
			unit.Entry(300).Told(selection.Allow{}),
			unit.Entry(400).Told(players.Give{To: c.one.ID}),
		}
	})
	if got := c.reached(c.one.ID); !only(got, 100) {
		t.Errorf("player one reaches %v, want the unit at 100 alone", got)
	}
	if got := c.reached(c.two.ID); !only(got, 200) {
		t.Errorf("player two reaches %v, want the unit at 200 alone", got)
	}
	if got := c.reached(control.Nobody); !only(got, 300) {
		t.Errorf("nobody reaches %v, want the ownerless unit at 300 alone", got)
	}
}

// A unit gives itself away and forbids its own selection mid-game by the same commands.
func TestGiveAndForbid_ChangeAUnitMidGame(t *testing.T) {
	c := newCamp(t, func(c *camp, unit kind.Of[float64]) []kind.Entry {
		return []kind.Entry{
			unit.Entry(100).Told(players.Give{To: c.one.ID}, selection.Allow{}),
			unit.Entry(200).Told(players.Give{To: c.one.ID}, selection.Allow{}),
		}
	})
	ids := c.ids()
	c.w.Commands().PutFrom(ids[100], players.Give{To: c.two.ID})
	c.w.Commands().PutFrom(ids[200], selection.Forbid{})
	c.tick()
	if got := c.reached(c.one.ID); !only(got) {
		t.Errorf("player one reaches %v, want none: one unit given away, the other forbidden", got)
	}
	if got := c.reached(c.two.ID); !only(got, 100) {
		t.Errorf("player two reaches %v, want the unit given to it, at 100", got)
	}
}

// Allow with Selected has the unit selected from the start.
func TestAllow_SelectedSelectsAtOnce(t *testing.T) {
	c := newCamp(t, func(c *camp, unit kind.Of[float64]) []kind.Entry {
		return []kind.Entry{
			unit.Entry(100).Told(players.Give{To: c.one.ID}, selection.Allow{Selected: true}),
			unit.Entry(200).Told(players.Give{To: c.one.ID}, selection.Allow{}),
		}
	})
	c.w.Commands().Put(c.one.ID, rule.Cast(c.haste).On(c.sel.Selected()))
	c.tick()
	ids := c.ids()
	if !c.haste.On(ids[100]) || c.haste.On(ids[200]) {
		t.Errorf("hasted: the unit at 100 %v, at 200 %v; want the one selected from the start alone", c.haste.On(ids[100]), c.haste.On(ids[200]))
	}
}

// An entry telling its entity a command no plugin in use carries out is refused as it is seeded.
func TestTold_ACommandNobodyCarriesOutIsRefused(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10},
	})
	kind.Define[float64](w.Kinds(), "unit", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
	})
	unit := kind.Named[float64](w.Kinds(), "unit")
	w.Seed(unit.Entry(0).Told(selection.Allow{})) // no selection plugin in use
	if err := w.Populate(); err == nil {
		t.Error("Populate took an entry told a command nobody carries out, want an error")
	}
}
