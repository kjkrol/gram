package world

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
)

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
				t.Errorf("clampStep(%v, %v) = %v, want (%v, %v)", c.step, c.max, got, c.wantX, c.wantY)
			}
		})
	}
}

func TestMoveSystem_CarriesSubUnitSpeedEveryTick(t *testing.T) {
	const tps = 60
	wm := testWorld()

	wm.populate(testKind(
		Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)},
		Velocity{Dir: geom.NewVec(1, 0), Value: 30},
	), []any{nil})

	var base goke.Comp[Base]
	var query *goke.Query
	ecs := goke.New()
	ecs.Setup(append(wm.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&base).Build()
	}})...)
	wm.RegSystems(ecs)
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { wm.RunPlan(rc, d); wm.clock.Replay(rc, d) })

	x := func() float64 {
		query.All()
		for query.Next() {
			return base.Slice(query.Cursor())[0].Pos.TopLeft.X
		}
		t.Fatal("the entity vanished")
		return 0
	}

	const step = 30 * (16666666.0 / 1e9)

	start := x()
	for tick := 1; tick <= 4; tick++ {
		ecs.Tick(time.Second / tps)
		want := start + step*float64(tick)
		if got := x(); math.Abs(got-want) > 1e-6 {
			t.Fatalf("after %d ticks the entity is at %v, want %v — half a unit per tick, every tick", tick, got, want)
		}
	}
}
