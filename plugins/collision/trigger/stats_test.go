package trigger_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/trigger"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
)

// overlapping runs the real collision engine, with s counting, over two overlapping sensors.
func overlapping(t *testing.T, s *trigger.ContactStats, ticks int) {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{
		Width: 1000, Height: 1000,
		BucketSize: 64,
	})
	if err != nil {
		t.Fatalf("aabbworld.NewSpace: %v", err)
	}

	ecs := goke.New()
	engine := collision.New(space, ecs)
	if err := engine.Hook(act.Trigger[collision.Meeting]("count contacts").Do(trigger.CountContacts(s))); err != nil {
		t.Fatalf("Hook: %v", err)
	}

	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var base goke.Comp[world.Base]
		var coll goke.Comp[collision.Collider]
		f := si.NewFactory(&base, &coll)
		f.Create(2)
		f.Next()
		for i, x := range []float64{100, 105} {
			box := plane.NewAABB(geom.NewVec(x, 100), 10, 10)
			base.Slice(&f.Cursor)[i].Pos = world.Position{AABB: box}
		}
	}})
	engine.RegSystems(ecs)
	ecs.SetPlan(engine.RunPlan)
	for range ticks {
		ecs.Tick(time.Millisecond)
	}
}

func TestCountContacts_CountsEachContactOnce(t *testing.T) {
	var s trigger.ContactStats

	overlapping(t, &s, 1)

	if s.Counter != 1 {
		t.Errorf("Counter = %d, want 1 for one pair in contact", s.Counter)
	}
}

func TestCountContacts_AccumulatesAcrossTicks(t *testing.T) {
	s := trigger.ContactStats{Counter: 5}

	overlapping(t, &s, 3)

	if s.Counter != 8 {
		t.Errorf("Counter = %d, want 8 — three more ticks in contact on top of the 5 it started with", s.Counter)
	}
}
