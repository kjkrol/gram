package motion

import (
	"cmp"
	"math"
	"math/rand"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
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
	side float32,
) (*goke.ECS, *goke.Query, testHandles) {
	t.Helper()
	ecs, q, h, _ := newTestWorldIn(t, testSpace(t), vel, side)
	return ecs, q, h
}

func newTestWorldIn(
	t *testing.T,
	space *aabbworld.Space,
	vel world.Velocity,
	side float32,
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
	if f32.Abs(p.TopLeft.X-0.01) > f32.Eps(0.01) {
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
	if f32.Abs(p.TopLeft.X-6) > f32.Eps(6) {
		t.Errorf("TopLeft.X = %v, want 6 (three ticks of two units)", p.TopLeft.X)
	}
}

func TestMoveSystem_Update_HandsTheSpaceEveryEntityWhereItNowIs(t *testing.T) {
	vel := world.Velocity{Dir: geom.NewVec(1, 0), Value: 100}
	ecs, _, _, space := newTestWorldIn(t, testSpace(t), vel, 5)

	at := func(x float32) int {
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
	for _, side := range []float32{2, 16, 100} {
		ecs, q, h := newTestWorld(t, vel, side)

		ecs.Tick(100 * time.Millisecond)

		p, _ := readFirst(t, q, h)
		if want := side / 2; f32.Abs(p.TopLeft.X-want) > f32.Eps(want) {
			t.Errorf("a %vx%v entity moved %v in one tick, want %v — half its own side",
				side, side, p.TopLeft.X, want)
		}
		if got := p.MaxStep(); got != side/2 {
			t.Errorf("a %vx%v entity's MaxStep = %v, want %v", side, side, got, side/2)
		}
	}
}

func TestClampStep_ScalesOnlyPastTheLimit(t *testing.T) {
	cases := []struct {
		name         string
		step         geom.Vec
		max          float32
		wantX, wantY float32
	}{
		{"within max is left alone", geom.NewVec(3, 4), 10, 3, 4},
		{"exactly at max is left alone", geom.NewVec(3, 4), 5, 3, 4},
		{"above max scales down, keeping the direction", geom.NewVec(6, 8), 5, 3, 4},
		{"a standstill has no direction to keep", geom.NewVec(0, 0), 5, 0, 0},
		{"a limit of nothing stops the step", geom.NewVec(3, 4), 0, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := clampStep(c.step, c.max)
			if f32.Abs(got.X-c.wantX) > f32.Eps(c.wantX) ||
				f32.Abs(got.Y-c.wantY) > f32.Eps(c.wantY) {
				t.Errorf("clampStep(%v, %v) = %v, want (%v, %v)", c.step, c.max, got, c.wantX,
					c.wantY)
			}
		})
	}
	byHypot := func(step geom.Vec, max float32) geom.Vec {
		mag := f32.Hypot(step.X, step.Y)
		if mag <= max || mag == 0 {
			return step
		}
		return geom.NewVec(step.X*max/mag, step.Y*max/mag)
	}
	rng := rand.New(rand.NewSource(7))
	for range 100 {
		step := geom.NewVec(float32(rng.Float64())*20-10, float32(rng.Float64())*20-10)
		max := float32(rng.Float64()) * 10
		got, want := clampStep(step, max), byHypot(step, max)
		if f32.Abs(got.X-want.X) > f32.Eps(want.X) || f32.Abs(got.Y-want.Y) > f32.Eps(want.Y) {
			t.Errorf("clampStep(%v, %v) = %v, want %v as Hypot has it", step, max, got, want)
		}
	}
}

// stepwise moves every box by Space.Move alone: what the move system's runs must match
type stepwise struct{ *moveSystem }

func (s stepwise) Update(cb *goke.CmdBuf, d time.Duration) {
	for s.moveQuery.Paced(d); s.moveQuery.Next(); {
		cursor, step := s.moveQuery.Cursor(), s.moveQuery.Dt()
		dt, reach := float32(step.Seconds()), float32(max(step/d, 1))
		bases, drifts := s.base.Slice(cursor), s.drift.Slice(cursor)
		marked := s.outside.Present(cursor)
		for i, id := range cursor.IDs {
			rate := bases[i].Vel.Delta()
			if drifts != nil {
				rate.X += drifts[i].X
				rate.Y += drifts[i].Y
			}
			by := clampStep(geom.NewVec(rate.X*dt, rate.Y*dt), bases[i].Pos.MaxStep()*reach)
			if by.X == 0 && by.Y == 0 {
				continue
			}
			if !s.space.Move(&bases[i].Pos.AABB, by) && !marked {
				cb.AddOne(id, s.outsideID, Outside{})
			}
		}
	}
	s.rebuild()
}

type lying struct {
	id      uid.UID64
	box     [8]uint32
	outside bool
}

// crowd is boxes of every size, speed, strafe and drift, near and in a band paced by four and
// beyond the bands, moved by the move system or by the stepwise one
type crowd struct {
	ecs     *goke.ECS
	all     *goke.Query
	base    goke.Comp[world.Base]
	outside goke.OptComp[Outside]
}

func newCrowd(t *testing.T, edges aabbworld.Edges, alone bool) *crowd {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{
		Width: 400, Height: 300, Edges: edges, BucketSize: 32,
	})
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(11))
	c := &crowd{ecs: goke.New()}
	var drift goke.Comp[Drift]
	c.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for k, f := range []*goke.Factory{
			si.NewFactory(&c.base),
			si.NewFactory(&c.base, &drift),
			si.NewFactory(&c.base, &goke.Comp[sector.Band2]{}),
			si.NewFactory(&c.base, &drift, &goke.Comp[sector.Band2]{}),
			si.NewFactory(&c.base, &goke.Comp[sector.Off]{}),
		} {
			f.Create(150)
			for f.Next() {
				for i := range c.base.Slice(&f.Cursor) {
					c.base.Slice(&f.Cursor)[i] = randomBase(rng, space)
				}
				if k%2 == 1 {
					for i := range drift.Slice(&f.Cursor) {
						drift.Slice(&f.Cursor)[i] = Drift{
							X: float32(rng.Float64())*200 - 100,
							Y: -40,
						}
					}
				}
			}
		}
		c.all = si.NewQueryBuilder(&c.base).Optional(&c.outside).Build()
	}})
	var steps uint64
	sys := newMoveSystem(space)
	sys.pace = goke.PacedBy(func() uint64 { return steps },
		goke.Never[sector.Off](), goke.Every[sector.Band2](4))
	var handle goke.Runnable
	if alone {
		handle = c.ecs.RegSys(stepwise{sys})
	} else {
		handle = c.ecs.RegSys(sys)
	}
	c.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(handle, d)
		ctx.Sync()
		steps++
	})
	return c
}

func randomBase(rng *rand.Rand, space *aabbworld.Space) world.Base {
	side := []float32{1, 4, 20}[rng.Intn(3)]
	at := geom.NewVec(float32(rng.Float64())*400, float32(rng.Float64())*300)
	box := plane.NewAABB(at, side, side*(0.5+float32(rng.Float64())))
	b := world.Base{Pos: world.Position{AABB: box}}
	space.Place(&b.Pos.AABB)
	if rng.Intn(6) == 0 {
		return b
	}
	angle := float32(rng.Float64()) * 2 * math.Pi
	b.Vel = world.Velocity{
		Dir:   geom.NewVec(f32.Cos(angle), f32.Sin(angle)),
		Value: float32(rng.Float64())*600 - 200,
	}
	if rng.Intn(3) == 0 {
		b.Vel.Side = float32(rng.Float64())*100 - 50
	}
	return b
}

func (c *crowd) state() []lying {
	var all []lying
	for c.all.All(); c.all.Next(); {
		cursor := c.all.Cursor()
		marked := c.outside.Present(cursor)
		for i, b := range c.base.Slice(cursor) {
			box := b.Pos.AABB
			all = append(all, lying{id: cursor.IDs[i], outside: marked, box: [8]uint32{
				math.Float32bits(box.TopLeft.X), math.Float32bits(box.TopLeft.Y),
				math.Float32bits(box.BottomRight.X), math.Float32bits(box.BottomRight.Y),
				math.Float32bits(box.Size.X), math.Float32bits(box.Size.Y),
				math.Float32bits(box.Overhang.X), math.Float32bits(box.Overhang.Y),
			}})
		}
	}
	slices.SortFunc(all, func(a, b lying) int { return cmp.Compare(a.id, b.id) })
	return all
}

func TestMoveSystem_MovesEveryBoxAsSpaceMoveDoesAlone(t *testing.T) {
	for _, edges := range []aabbworld.Edges{
		0, aabbworld.Torus, aabbworld.OpenX | aabbworld.OpenY, aabbworld.WrapX | aabbworld.OpenY,
	} {
		runs, alone := newCrowd(t, edges, false), newCrowd(t, edges, true)
		var got []lying
		for tick := range 200 {
			runs.ecs.Tick(time.Second / 60)
			alone.ecs.Tick(time.Second / 60)
			got = runs.state()
			if want := alone.state(); !slices.Equal(got, want) {
				i := 0
				for i < min(len(got), len(want)) && got[i] == want[i] {
					i++
				}
				t.Fatalf("edges %b, tick %d: %d boxes, %d stepwise, apart from %d on",
					edges, tick, len(got), len(want), i)
			}
		}
		left := 0
		for _, m := range got {
			if m.outside {
				left++
			}
		}
		if open := edges&(aabbworld.OpenX|aabbworld.OpenY) != 0; open != (left > 0) {
			t.Errorf("edges %b: %d boxes outside after 200 ticks", edges, left)
		}
	}
}
