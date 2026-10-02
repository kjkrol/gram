package world

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
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

// testHandles bundles the component handles a test needs to read back after ticking.
type testHandles struct {
	base *goke.Comp[Base]
}

// newTestWorld seeds one side x side entity moving at vel under the world's move system.
func newTestWorld(t *testing.T, vel Velocity, side float64) (*goke.ECS, *goke.Query, testHandles) {
	t.Helper()
	ecs, q, h, _ := newTestWorldIn(t, testSpace(t), vel, side)
	return ecs, q, h
}

func newTestWorldIn(t *testing.T, space *aabbworld.Space, vel Velocity, side float64) (*goke.ECS, *goke.Query, testHandles, *aabbworld.Space) {
	t.Helper()

	ecs := goke.New()
	var base goke.Comp[Base]
	var q *goke.Query
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base)
		f.Create(1)
		f.Next()
		base.Slice(&f.Cursor)[0] = Base{
			Pos: Position{AABB: plane.NewAABB(geom.NewVec(0, 0), side, side)},
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

func readFirst(t *testing.T, q *goke.Query, h testHandles) (Position, Velocity) {
	t.Helper()
	q.All()
	for q.Next() {
		b := h.base.Slice(q.Cursor())
		if len(b) > 0 {
			return b[0].Pos, b[0].Vel
		}
	}
	t.Fatal("expected to find the seeded entity")
	return Position{}, Velocity{}
}

func TestMoveSystem_Update_SubPixelAccumulatesWithoutMoving(t *testing.T) {
	vel := Velocity{Dir: geom.NewVec(1, 0), Value: 1}
	ecs, q, h := newTestWorld(t, vel, 5)

	ecs.Tick(10 * time.Millisecond)

	p, _ := readFirst(t, q, h)
	if math.Abs(p.TopLeft.X-0.01) > 1e-9 {
		t.Errorf("TopLeft.X = %v, want 0.01 — the step is a hundredth of a unit and it should land there", p.TopLeft.X)
	}
}

func TestMoveSystem_Update_TranslatesWholeUnits(t *testing.T) {
	vel := Velocity{Dir: geom.NewVec(1, 0), Value: 100}
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
	vel := Velocity{Dir: geom.NewVec(1, 0), Value: 100}
	ecs, _, _, space := newTestWorldIn(t, testSpace(t), vel, 5)

	at := func(x float64) int {
		return space.Query(geom.NewAABB(geom.NewVec(x, 0), geom.NewVec(x+5, 5)), aabbworld.AnyCapability, func(uid.UID64) {})
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
	vel := Velocity{Dir: geom.NewVec(1, 0), Value: 1000}
	for _, side := range []float64{2, 16, 100} {
		ecs, q, h := newTestWorld(t, vel, side)

		ecs.Tick(100 * time.Millisecond)

		p, _ := readFirst(t, q, h)
		if want := side / 2; math.Abs(p.TopLeft.X-want) > 1e-9 {
			t.Errorf("a %vx%v entity moved %v in one tick, want %v — half its own side", side, side, p.TopLeft.X, want)
		}
		if got := p.MaxStep(); got != side/2 {
			t.Errorf("a %vx%v entity's MaxStep = %v, want %v", side, side, got, side/2)
		}
	}
}

func TestPosition_MaxSpeed_FollowsTheShorterSideAndTheTickRate(t *testing.T) {
	p := Position{AABB: plane.NewAABB(geom.NewVec(0, 0), 40, 10)}

	if got := p.MaxStep(); got != 5 {
		t.Errorf("MaxStep of a 40x10 entity = %v, want 5 — half the shorter side", got)
	}
	if got := p.MaxSpeed(60); got != 300 {
		t.Errorf("MaxSpeed at 60 ticks a second = %v, want 300", got)
	}
}
