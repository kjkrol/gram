package world

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

type left struct{}

func (left) Command() {}

type lefts struct{ command.Queue[left] }

func (l *lefts) Queues() []command.CommandQueue     { return []command.CommandQueue{&l.Queue} }
func (l *lefts) DefaultBindings() []control.Binding { return nil }

func (l *lefts) heard() []uid.UID64 {
	var who []uid.UID64
	l.Drain(func(i command.Issued[left]) { who = append(who, i.Entity) })
	return who
}

type exitWorld struct {
	p            *Plugin
	ecs          *goke.ECS
	all, outside *goke.Query
	base         goke.Comp[Base]
	lefts        lefts
}

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
		if err := ew.p.Hook(act.Branch[Leaving]{
			Name: "left",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(left{}),
		}); err != nil {
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

func (ew *exitWorld) ticks(n int) {
	for range n {
		ew.ecs.Tick(time.Second / 60)
	}
}

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
