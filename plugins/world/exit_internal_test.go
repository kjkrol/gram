package world

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/uid"
)

// leaving is a world with one 10x10 entity heading east, four ticks from wholly crossing the edge,
// with rules registered on it.
func leaving(t *testing.T, edges aabbworld.Edges, rules ...plugin.Rule) (*Plugin, *goke.ECS, *goke.Query, *goke.Query) {
	t.Helper()
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1000, Height: 1000, Edges: edges},
		Entities: EntitiesCfg{MaxCount: 10, MinSize: 1, MaxSize: 100},
	})
	if err := p.Hook(rules...); err != nil {
		t.Fatal(err)
	}
	wm := p.module
	wm.populate(testKind(
		Position{AABB: plane.NewAABB(geom.NewVec(985, 500), 10, 10)},
		Velocity{Dir: geom.NewVec(1, 0), Value: 300},
	), []any{nil})

	var base, marked goke.Comp[Base]
	var query, outside *goke.Query
	ecs := goke.New()
	ecs.Setup(append(wm.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&base).Build()
		outside = si.NewQueryBuilder(&marked).Include(goke.Include[Outside]()).Build()
	}})...)
	wm.RegSystems(ecs)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		wm.RunPlan(ctx, d)
		wm.clock.Replay(ctx, d)
		ctx.Sync()
	})
	return p, ecs, query, outside
}

// everywhere is how many indexed pieces a query over the whole world finds.
func everywhere(space *aabbworld.Space) int {
	return space.Query(geom.NewAABBAt(geom.NewVec(0, 0), 1000, 1000), aabbworld.AnyCapability, func(uid.UID64) {})
}

func TestExit_AnEntityLeavingByAnOpenEdgeIsDespawnedByDefault(t *testing.T) {
	p, ecs, query, _ := leaving(t, aabbworld.OpenX)

	ecs.Tick(time.Second / 60)
	ecs.Tick(time.Second / 60)
	if got := len(living(query)); got != 1 {
		t.Fatalf("%d entities alive while part of it is still inside, want 1", got)
	}
	ecs.Tick(time.Second / 60)
	ecs.Tick(time.Second / 60)
	if got := len(living(query)); got != 0 {
		t.Errorf("%d entities alive once it has wholly left, want 0", got)
	}
	if got := everywhere(p.module.space); got != 0 {
		t.Errorf("the index still holds %d entities", got)
	}
}

// hears is an Each of Leaving that appends every id it is told of to dst.
func hears(dst *[]uid.UID64) plugin.Rule {
	return host.Each(func(_ plugin.Tick, _ *Appearance, l Leaving) { *dst = append(*dst, l.ID) })
}

func TestExit_ALeavingRuleHearsOfTheLeaverEveryTickItIsOutAndKeepsItAlive(t *testing.T) {
	var heard []uid.UID64
	_, ecs, query, _ := leaving(t, aabbworld.OpenX, hears(&heard))

	for range 6 {
		ecs.Tick(time.Second / 60)
	}
	alive := living(query)
	if len(alive) != 1 {
		t.Fatalf("%d entities alive, want the leaver kept for the rule to deal with", len(alive))
	}
	if len(heard) != 3 {
		t.Errorf("heard %v over six ticks, want the leaver on each of the three ticks it was out", heard)
	}
	for _, id := range heard {
		if !alive[id] {
			t.Errorf("heard of %d, which is not the living leaver", id)
		}
	}
}

func TestExit_ALeaverPutBackInsideLosesItsMark(t *testing.T) {
	var heard []uid.UID64
	back := host.Each(func(_ plugin.Tick, _ *Appearance, l Leaving) {
		b := l.Base
		heard = append(heard, l.ID)
		b.Pos.AABB = plane.NewAABB(geom.NewVec(500, 500), 10, 10)
		b.Vel.Value = 0
	})
	_, ecs, query, outside := leaving(t, aabbworld.OpenX, back)

	for range 6 {
		ecs.Tick(time.Second / 60)
	}
	if len(heard) != 1 {
		t.Errorf("heard %v, want the leaver once: put back inside, it is no longer Outside", heard)
	}
	if got := len(living(query)); got != 1 {
		t.Fatalf("%d entities alive, want 1", got)
	}
	if marked := len(living(outside)); marked != 0 {
		t.Errorf("%d entities still carry Outside, want none", marked)
	}
}

func TestExit_AClosedEdgeStopsTheEntityWhole(t *testing.T) {
	var heard []uid.UID64
	p, ecs, query, _ := leaving(t, 0, hears(&heard))

	for range 6 {
		ecs.Tick(time.Second / 60)
	}
	if len(heard) != 0 {
		t.Errorf("heard %v in a closed world, want nobody", heard)
	}
	if got := len(living(query)); got != 1 {
		t.Fatalf("%d entities alive, want 1", got)
	}
	if got := everywhere(p.module.space); got != 1 {
		t.Errorf("the index holds %d entities, want the one resting against the edge", got)
	}
}
