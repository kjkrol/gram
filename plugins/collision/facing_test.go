package collision_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
)

type facer struct {
	x   float32
	dir geom.Vec
}

func TestMeeting_FacingIsTheWordOfTheOneTurnedTowardsTheOther(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 3, MinSize: 10, MaxSize: 10},
	})
	var orders heards
	if err := w.Carry(&orders); err != nil {
		t.Fatal(err)
	}
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	if err := c.Hook(act.Branch[collision.Meeting]{
		Name: "facing",
		When: cond.Self.If(collision.Facing(math.Pi / 4)),
		Then: act.Self.Do(heard{Rule: "facing"}),
	}); err != nil {
		t.Fatal(err)
	}
	// a lever, a sensor, at 100; one facing it from the west, one with its back to it
	kind.Define[facer](w.Kinds(), "thing", kind.Spec{
		comp.Load(func(f facer) world.Position { return posAt(f.x, 100, 10, 10) }),
		comp.Load(func(f facer) world.Velocity { return world.Velocity{Dir: f.dir} }),
		comp.Const(collision.Collider{}),
	})
	thing := kind.Named[facer](w.Kinds(), "thing")
	w.Write().Seed(thing.Entry(facer{x: 100}), thing.Entry(facer{x: 96, dir: geom.NewVec(1, 0)}),
		thing.Entry(facer{x: 104, dir: geom.NewVec(1, 0)}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ecs := collisiontest.Start(t, w, m, c)
	ecs.Tick(time.Second / 60)
	var told []float32
	for _, h := range orders.given() {
		told = append(told, 0)
		_ = h
	}
	if len(told) != 1 {
		t.Fatalf("facing told %d times, want once: the one west of the lever, turned east", len(told))
	}
}
