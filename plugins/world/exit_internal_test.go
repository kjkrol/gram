package world

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// left is the command the tests' Leaving rule gives.
type left struct{}

// lefts is where the left commands land, for the world to carry.
type lefts struct{ control.Queue[left] }

func (l *lefts) Queues() []control.CommandQueue     { return []control.CommandQueue{&l.Queue} }
func (l *lefts) DefaultBindings() []control.Binding { return nil }

// heard is who gave a left since the last time.
func (l *lefts) heard() []uid.UID64 {
	var who []uid.UID64
	l.Drain(func(i control.Issued[left]) { who = append(who, i.Entity) })
	return who
}

// exitWorld is a world with one 10x10 entity heading east, four ticks from wholly crossing the
// edge: every entity, those Outside, and, with a Leaving rule hooked, whom it fired for.
type exitWorld struct {
	p            *Plugin
	ecs          *goke.ECS
	all, outside *goke.Query
	base         goke.Comp[Base]
	lefts        lefts
}

// leaving builds the exitWorld, with a Leaving rule giving a left when hooked.
func leaving(t *testing.T, edges aabbworld.Edges, hooked bool) *exitWorld {
	t.Helper()
	ew := &exitWorld{p: NewPlugin(Config{
		Space:    SpaceCfg{Width: 1000, Height: 1000, Edges: edges},
		Entities: EntitiesCfg{MaxCount: 10, MinSize: 1, MaxSize: 100},
	})}
	if err := ew.p.Carry(&ew.lefts); err != nil {
		t.Fatal(err)
	}
	if hooked {
		if err := ew.p.Hook(rule.Then[Leaving]("left", rule.All, rule.Order(left{}))); err != nil {
			t.Fatal(err)
		}
	}
	wm := ew.p.module
	wm.populate(testKind(
		Position{AABB: plane.NewAABB(geom.NewVec(985, 500), 10, 10)},
		Velocity{Dir: geom.NewVec(1, 0), Value: 300},
	), []any{nil})

	var marked goke.Comp[Base]
	ew.ecs = goke.New()
	ew.ecs.Setup(append(wm.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		ew.all = si.NewQueryBuilder(&ew.base).Build()
		ew.outside = si.NewQueryBuilder(&marked).Include(goke.Include[Outside]()).Build()
	}})...)
	wm.RegSystems(ew.ecs)
	ew.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		wm.RunPlan(ctx, d)
		wm.clock.Replay(ctx, d)
		ctx.Sync()
	})
	return ew
}

// ticks steps the world n times.
func (ew *exitWorld) ticks(n int) {
	for range n {
		ew.ecs.Tick(time.Second / 60)
	}
}

// everywhere is how many indexed pieces a query over the whole world finds.
func everywhere(space *aabbworld.Space) int {
	return space.Query(geom.NewAABBAt(geom.NewVec(0, 0), 1000, 1000), aabbworld.AnyCapability, func(uid.UID64) {})
}

func TestExit_AnEntityLeavingByAnOpenEdgeIsDespawnedByDefault(t *testing.T) {
	ew := leaving(t, aabbworld.OpenX, false)

	ew.ticks(2)
	if got := len(living(ew.all)); got != 1 {
		t.Fatalf("%d entities alive while part of it is still inside, want 1", got)
	}
	ew.ticks(2)
	if got := len(living(ew.all)); got != 0 {
		t.Errorf("%d entities alive once it has wholly left, want 0", got)
	}
	if got := everywhere(ew.p.module.space); got != 0 {
		t.Errorf("the index still holds %d entities", got)
	}
}

func TestExit_ALeavingRuleHearsOfTheLeaverEveryTickItIsOutAndKeepsItAlive(t *testing.T) {
	ew := leaving(t, aabbworld.OpenX, true)

	ew.ticks(6)
	alive := living(ew.all)
	if len(alive) != 1 {
		t.Fatalf("%d entities alive, want the leaver kept for the rule to deal with", len(alive))
	}
	heard := ew.lefts.heard()
	if len(heard) != 3 {
		t.Errorf("heard %v over six ticks, want the leaver on each of the three ticks it was out", heard)
	}
	for _, id := range heard {
		if !alive[id] {
			t.Errorf("heard of %d, which is not the living leaver", id)
		}
	}
}

// Put back inside the tick it is first heard of, the leaver loses its mark in the next pass —
// heard once more there, never after.
func TestExit_ALeaverPutBackInsideLosesItsMark(t *testing.T) {
	ew := leaving(t, aabbworld.OpenX, true)

	var heard []uid.UID64
	for range 6 {
		ew.ticks(1)
		got := ew.lefts.heard()
		heard = append(heard, got...)
		if len(got) == 0 {
			continue
		}
		for ew.all.All(); ew.all.Next(); {
			for i := range ew.base.Slice(ew.all.Cursor()) {
				b := &ew.base.Slice(ew.all.Cursor())[i]
				b.Pos.AABB = plane.NewAABB(geom.NewVec(500, 500), 10, 10)
				b.Vel.Value = 0
			}
		}
	}
	if len(heard) != 2 {
		t.Errorf("heard %v, want the leaver twice: out, then in the pass that finds it back inside", heard)
	}
	if got := len(living(ew.all)); got != 1 {
		t.Fatalf("%d entities alive, want 1", got)
	}
	if marked := len(living(ew.outside)); marked != 0 {
		t.Errorf("%d entities still carry Outside, want none", marked)
	}
}

func TestExit_AClosedEdgeStopsTheEntityWhole(t *testing.T) {
	ew := leaving(t, 0, true)

	ew.ticks(6)
	if heard := ew.lefts.heard(); len(heard) != 0 {
		t.Errorf("heard %v in a closed world, want nobody", heard)
	}
	if got := len(living(ew.all)); got != 1 {
		t.Fatalf("%d entities alive, want 1", got)
	}
	if got := everywhere(ew.p.module.space); got != 1 {
		t.Errorf("the index holds %d entities, want the one resting against the edge", got)
	}
}
