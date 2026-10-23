package motion_test

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
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/internal/motiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type left struct{}

func (left) Command() {}

var _ command.Command = left{}

type lefts struct{ command.Queue[left] }

func (l *lefts) Queues() []command.CommandQueue     { return []command.CommandQueue{&l.Queue} }
func (l *lefts) DefaultBindings() []control.Binding { return nil }

func (l *lefts) heard() []uid.UID64 {
	var who []uid.UID64
	l.Drain(func(i command.Issued[left]) { who = append(who, i.Entity) })
	return who
}

type exitRig struct {
	*rig
	outside *goke.Query
	lefts   lefts
}

// a thing heading out by the right edge, 300 units a second, a rule of Leaving heard if heeded
func leaving(t *testing.T, edges aabbworld.Edges, heeded bool) *exitRig {
	t.Helper()
	var rules []act.Rule
	if heeded {
		rules = append(rules, act.Branch[motion.Leaving]{
			Name: "left",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(left{}),
		})
	}
	er := &exitRig{}
	er.rig = newRig(t, edges, box(985, 500, 10), world.Velocity{Dir: geom.NewVec(1, 0), Value: 300},
		nil, rules...)
	if err := er.w.Carry(&er.lefts); err != nil {
		t.Fatal(err)
	}
	var marked goke.Comp[world.Base]
	er.ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		er.outside = si.NewQueryBuilder(&marked).Include(goke.Include[motion.Outside]()).Build()
	}})
	return er
}

func everywhere(space *aabbworld.Space) int {
	return space.Query(geom.NewAABBAt(geom.NewVec(0, 0), 1000, 1000), aabbworld.AnyCapability,
		func(uid.UID64) {})
}

func TestExit_AnEntityLeavingByAnOpenEdgeIsDespawnedByDefault(t *testing.T) {
	er := leaving(t, aabbworld.OpenX, false)

	er.ticks(2)
	if got := len(living(er.all)); got != 1 {
		t.Fatalf("%d entities alive while part of it is still inside, want 1", got)
	}
	er.ticks(2)
	if got := len(living(er.all)); got != 0 {
		t.Errorf("%d entities alive once it has wholly left, want 0", got)
	}
	if got := everywhere(er.w.Write().Space()); got != 0 {
		t.Errorf("the index still holds %d entities", got)
	}
}

func TestExit_ALeavingRuleHearsOfTheLeaverEveryTickItIsOutAndKeepsItAlive(t *testing.T) {
	er := leaving(t, aabbworld.OpenX, true)

	er.ticks(6)
	alive := living(er.all)
	if len(alive) != 1 {
		t.Fatalf("%d entities alive, want the leaver kept for the rule to deal with", len(alive))
	}
	heard := er.lefts.heard()
	if len(heard) != 3 {
		t.Errorf("heard %v over six ticks, want the leaver on each of the three ticks it was out",
			heard)
	}
	for _, id := range heard {
		if !alive[id] {
			t.Errorf("heard of %d, which is not the living leaver", id)
		}
	}
}

func TestExit_ALeaverPutBackInsideLosesItsMark(t *testing.T) {
	er := leaving(t, aabbworld.OpenX, true)

	var heard []uid.UID64
	for range 6 {
		er.ticks(1)
		got := er.lefts.heard()
		heard = append(heard, got...)
		if len(got) == 0 {
			continue
		}
		for er.all.All(); er.all.Next(); {
			for i := range er.base.Slice(er.all.Cursor()) {
				b := &er.base.Slice(er.all.Cursor())[i]
				b.Pos.AABB = plane.NewAABB(geom.NewVec(500, 500), 10, 10)
				b.Vel.Value = 0
			}
		}
	}
	if len(heard) != 2 {
		t.Errorf("heard %v, want the leaver twice: out, then in the pass that finds it back inside",
			heard)
	}
	if got := len(living(er.all)); got != 1 {
		t.Fatalf("%d entities alive, want 1", got)
	}
	if marked := len(living(er.outside)); marked != 0 {
		t.Errorf("%d entities still carry Outside, want none", marked)
	}
}

func TestExit_AClosedEdgeStopsTheEntityWhole(t *testing.T) {
	er := leaving(t, 0, true)

	er.ticks(6)
	if heard := er.lefts.heard(); len(heard) != 0 {
		t.Errorf("heard %v in a closed world, want nobody", heard)
	}
	if got := len(living(er.all)); got != 1 {
		t.Fatalf("%d entities alive, want 1", got)
	}
	if got := everywhere(er.w.Write().Space()); got != 1 {
		t.Errorf("the index holds %d entities, want the one resting against the edge", got)
	}
}

func TestExit_AnEntityLeftByAnotherPluginIsMotionsLeaver(t *testing.T) {
	r := newRig(t, aabbworld.OpenX, box(500, 500, 10), world.Velocity{}, nil)
	var ids []uid.UID64
	var base goke.Comp[world.Base]
	push := r.ecs.RegSys(goke.SystemFn{
		OnInit: func(si *goke.SysInit) {
			q := si.NewQueryBuilder(&base).Build()
			for q.All(); q.Next(); {
				ids = append(ids, q.Cursor().IDs...)
			}
		},
		OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
			for r.all.All(); r.all.Next(); {
				r.base.Slice(r.all.Cursor())[0].Pos.AABB = plane.NewAABB(geom.NewVec(1500, 500),
					10, 10)
			}
			r.m.Write().Leave(cb, ids[0])
		},
	})
	step := motiontest.Step(r.w, r.m)
	pushed := false
	r.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		step(rc, d)
		if !pushed {
			rc.Run(push, d)
			rc.Sync()
			pushed = true
		}
	})

	r.ticks(1)
	if got := len(living(r.all)); got != 1 {
		t.Fatalf("%d entities alive the step it was put out, want it still there", got)
	}
	r.ticks(1)
	if got := len(living(r.all)); got != 0 {
		t.Errorf("%d entities alive a step after it was left, want it despawned by motion", got)
	}
}
