package motion

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

func testSpace(t *testing.T) *aabbworld.Space {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{
		Width: 1000, Height: 1000,
		BucketSize: 64,
	})
	if err != nil {
		t.Fatalf("aabbworld.NewSpace: %v", err)
	}
	return space
}

type testHandles struct {
	base *goke.Comp[world.Base]
}

func newTestWorld(
	t *testing.T,
	vel world.Velocity,
	side float64,
) (*goke.ECS, *goke.Query, testHandles) {
	t.Helper()
	ecs, q, h, _ := newTestWorldIn(t, testSpace(t), vel, side)
	return ecs, q, h
}

func newTestWorldIn(
	t *testing.T,
	space *aabbworld.Space,
	vel world.Velocity,
	side float64,
) (*goke.ECS, *goke.Query, testHandles, *aabbworld.Space) {
	t.Helper()

	ecs := goke.New()
	var base goke.Comp[world.Base]
	var q *goke.Query
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base)
		f.Create(1)
		f.Next()
		base.Slice(&f.Cursor)[0] = world.Base{
			Pos: world.Position{AABB: plane.NewAABB(geom.NewVec(0, 0), side, side)},
			Vel: vel,
		}

		q = si.NewQueryBuilder(&base).Build()
	}})

	sys := newMoveSystem(space)
	handle := ecs.RegSys(sys)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(handle, d)
		ctx.Sync()
	})

	return ecs, q, testHandles{base: &base}, space
}

func readFirst(t *testing.T, q *goke.Query, h testHandles) (world.Position, world.Velocity) {
	t.Helper()
	q.All()
	for q.Next() {
		b := h.base.Slice(q.Cursor())
		if len(b) > 0 {
			return b[0].Pos, b[0].Vel
		}
	}
	t.Fatal("expected to find the seeded entity")
	return world.Position{}, world.Velocity{}
}

func TestMoveSystem_Update_SubPixelAccumulatesWithoutMoving(t *testing.T) {
	vel := world.Velocity{Dir: geom.NewVec(1, 0), Value: 1}
	ecs, q, h := newTestWorld(t, vel, 5)

	ecs.Tick(10 * time.Millisecond)

	p, _ := readFirst(t, q, h)
	if math.Abs(p.TopLeft.X-0.01) > 1e-9 {
		t.Errorf("TopLeft.X = %v, want 0.01 — the step is a hundredth of a unit and it should "+
			"land there", p.TopLeft.X)
	}
}

func TestMoveSystem_Update_TranslatesWholeUnits(t *testing.T) {
	vel := world.Velocity{Dir: geom.NewVec(1, 0), Value: 100}
	ecs, q, h := newTestWorld(t, vel, 5)

	for range 3 {
		ecs.Tick(20 * time.Millisecond)
	}

	p, _ := readFirst(t, q, h)
	if math.Abs(p.TopLeft.X-6) > 1e-9 {
		t.Errorf("TopLeft.X = %v, want 6 (three ticks of two units)", p.TopLeft.X)
	}
}

func TestMoveSystem_Update_HandsTheSpaceEveryEntityWhereItNowIs(t *testing.T) {
	vel := world.Velocity{Dir: geom.NewVec(1, 0), Value: 100}
	ecs, _, _, space := newTestWorldIn(t, testSpace(t), vel, 5)

	at := func(x float64) int {
		box := geom.NewAABB(geom.NewVec(x, 0), geom.NewVec(x+5, 5))
		return space.Query(box, aabbworld.AnyCapability, func(uid.UID64) {})
	}
	if got := at(0); got != 0 {
		t.Fatalf("the space holds %d entities before the first tick, want none", got)
	}
	ecs.Tick(20 * time.Millisecond)
	if got := at(2); got != 1 {
		t.Errorf("the space finds %d entities where the entity moved to, want 1", got)
	}
	ecs.Tick(20 * time.Millisecond)
	if got := at(4); got != 1 {
		t.Errorf("the space finds %d entities after the second move, want 1", got)
	}
	if got := at(-8); got != 0 {
		t.Errorf("the space still finds %d entities where the entity was, want 0", got)
	}
}

func TestMoveSystem_Update_ClampsTheStepToHalfTheEntitysOwnSide(t *testing.T) {
	vel := world.Velocity{Dir: geom.NewVec(1, 0), Value: 1000}
	for _, side := range []float64{2, 16, 100} {
		ecs, q, h := newTestWorld(t, vel, side)

		ecs.Tick(100 * time.Millisecond)

		p, _ := readFirst(t, q, h)
		if want := side / 2; math.Abs(p.TopLeft.X-want) > 1e-9 {
			t.Errorf("a %vx%v entity moved %v in one tick, want %v — half its own side",
				side, side, p.TopLeft.X, want)
		}
		if got := p.MaxStep(); got != side/2 {
			t.Errorf("a %vx%v entity's MaxStep = %v, want %v", side, side, got, side/2)
		}
	}
}

func TestClampStep(t *testing.T) {
	cases := []struct {
		name         string
		step         geom.Vec
		max          float64
		wantX, wantY float64
	}{
		{"within max is left alone", geom.NewVec(3, 4), 10, 3, 4},
		{"exactly at max is left alone", geom.NewVec(3, 4), 5, 3, 4},
		{"above max scales down, keeping the direction", geom.NewVec(6, 8), 5, 3, 4},
		{"a standstill has no direction to keep", geom.NewVec(0, 0), 5, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := clampStep(c.step, c.max)
			if math.Abs(got.X-c.wantX) > 1e-12 || math.Abs(got.Y-c.wantY) > 1e-12 {
				t.Errorf("clampStep(%v, %v) = %v, want (%v, %v)", c.step, c.max, got, c.wantX,
					c.wantY)
			}
		})
	}
}
