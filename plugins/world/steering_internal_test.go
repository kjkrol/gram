package world

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/rule"
	"github.com/kjkrol/gram/plugins/world/steering"
)

var (
	east  = geom.NewVec(1.0, 0.0)
	north = geom.NewVec(0.0, 1.0)
)

// steerTicks spawns one entity heading start, carrying st, and reports its heading per tick.
func steerTicks(t *testing.T, st steering.Steering, start geom.Vec, n int) []geom.Vec {
	t.Helper()

	wm := testWorld()
	wm.populate(testKind(
		Position{AABB: plane.NewAABB(geom.NewVec(500, 500), 10, 10)},
		Velocity{Dir: start, Value: 1},
		comp.Const(st),
	), []any{nil})

	var base goke.Comp[Base]
	var query *goke.Query
	ecs := goke.New()
	ecs.Setup(append(wm.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&base).Build()
	}})...)
	wm.RegSystems(ecs)
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { wm.RunPlan(rc, d); wm.clock.Replay(rc, d) })

	var out []geom.Vec
	for range n {
		ecs.Tick(time.Second / 60)
		query.All()
		for query.Next() {
			for _, b := range base.Slice(query.Cursor()) {
				out = append(out, b.Vel.Dir)
			}
		}
	}
	return out
}

func heading(v geom.Vec) float64 { return math.Atan2(v.Y, v.X) }

func TestSteering_RequestIsRefusedWhileStillReacting(t *testing.T) {
	s := &steering.Steering{Reflex: 3}

	if !s.Request(east) {
		t.Fatal("first Request refused on an idle Steering")
	}
	if s.Request(north) {
		t.Error("second Request taken while the first was still being reacted to")
	}
	if s.Pending != east {
		t.Errorf("Pending = %v, want the first request kept", s.Pending)
	}
}

func TestSteering_RequestNormalisesWhateverItIsHanded(t *testing.T) {
	s := &steering.Steering{}
	s.Request(geom.NewVec(3.0, 4.0))

	if n := math.Hypot(s.Want.X, s.Want.Y); math.Abs(n-1) > 1e-12 {
		t.Errorf("|Want| = %v, want 1", n)
	}
	if math.Abs(heading(s.Want)-math.Atan2(4, 3)) > 1e-12 {
		t.Errorf("Want points at %v, want the direction it was handed", heading(s.Want))
	}
}

func TestSteering_ReflexHoldsTheTurnBack(t *testing.T) {
	dirs := steerTicks(t, steering.Steering{Pending: east, Reflex: 2, Delay: 2}, north, 3)

	if dirs[0] != north || dirs[1] != north {
		t.Errorf("headings %v, %v during the reflex window, want both still north", dirs[0], dirs[1])
	}
	if dirs[2] != east {
		t.Errorf("heading %v once the reflex ran out, want east", dirs[2])
	}
}

func TestSteering_TurnRateCapsTheSwing(t *testing.T) {
	const rate = 0.1
	dirs := steerTicks(t, steering.Steering{Want: east, TurnRate: rate}, north, 3)

	from := heading(north)
	for i, d := range dirs {
		want := from - rate*float64(i+1)
		if math.Abs(heading(d)-want) > 1e-9 {
			t.Errorf("tick %d heading %.4f, want %.4f — one rate step per tick", i+1, heading(d), want)
		}
	}
	if heading(dirs[2]) <= 0 {
		t.Error("reached the target within three ticks, so the rate was not capping anything")
	}
}

func TestSteering_NoRateSwingsAllTheWayAtOnce(t *testing.T) {
	if dirs := steerTicks(t, steering.Steering{Want: east}, north, 1); dirs[0] != east {
		t.Errorf("heading %v after one tick with no TurnRate, want east", dirs[0])
	}
}

// The last step lands on the target exactly rather than overshooting it.
func TestSteering_LastStepSettlesOnTheTarget(t *testing.T) {
	dirs := steerTicks(t, steering.Steering{Want: east, TurnRate: 1.0}, north, 2)

	if heading(dirs[0]) <= 0 {
		t.Fatalf("heading %.4f after one tick, want the turn still under way", heading(dirs[0]))
	}
	if dirs[1] != east {
		t.Errorf("heading %v after the second tick, want east exactly", dirs[1])
	}
}

func TestSteering_StationaryEntityTakesTheHeadingWhole(t *testing.T) {
	var stationary geom.Vec
	if dirs := steerTicks(t, steering.Steering{Want: east, TurnRate: 0.01}, stationary, 1); dirs[0] != east {
		t.Errorf("heading %v after one tick from a standstill, want east", dirs[0])
	}
}

func TestSteering_LeavesHeadingAloneWithNoRequest(t *testing.T) {
	if dirs := steerTicks(t, steering.Steering{}, north, 2); dirs[0] != north || dirs[1] != north {
		t.Errorf("headings %v, want north throughout", dirs)
	}
}

// asking is a behavior that renews the same request every tick, the way any
// behavior watching a lasting stimulus does.
type asking struct {
	towards geom.Vec
	query   *goke.Query
	steer   goke.Comp[steering.Steering]
}

func (a *asking) Init(si *goke.SysInit) { a.query = si.NewQueryBuilder(&a.steer).Build() }

func (a *asking) Update(*goke.CmdBuf, time.Duration) {
	a.query.All()
	for a.query.Next() {
		steers := a.steer.Slice(a.query.Cursor())
		for i := range steers {
			steers[i].Request(a.towards)
		}
	}
}

func TestSteering_LastingStimulusStillTurnsTheEntity(t *testing.T) {
	wm := testWorld()
	wm.Hook(&asking{towards: north})
	wm.populate(testKind(
		Position{AABB: plane.NewAABB(geom.NewVec(500, 500), 10, 10)},
		Velocity{Dir: east, Value: 1},
		comp.Const(steering.Steering{Reflex: 3, TurnRate: 0.12}),
	), []any{nil})

	var base goke.Comp[Base]
	var query *goke.Query
	ecs := goke.New()
	ecs.Setup(append(wm.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&base).Build()
	}})...)
	wm.RegSystems(ecs)
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { wm.RunPlan(rc, d); wm.clock.Replay(rc, d) })
	for range 30 {
		ecs.Tick(time.Second / 60)
	}

	query.All()
	for query.Next() {
		for _, b := range base.Slice(query.Cursor()) {
			if b.Vel.Dir != north {
				t.Errorf("heading %v after 30 ticks of being asked north, want north — a quarter turn takes 14", b.Vel.Dir)
			}
		}
	}
}

func TestSteering_KeepsActingOnTheLastDecisionWhileReacting(t *testing.T) {
	const rate = 0.1
	dirs := steerTicks(t, steering.Steering{Want: east, Pending: north, Reflex: 3, Delay: 3, TurnRate: rate}, north, 2)

	for i, d := range dirs {
		if want := heading(north) - rate*float64(i+1); math.Abs(heading(d)-want) > 1e-9 {
			t.Errorf("tick %d heading %.4f, want %.4f — still swinging east while north waits its turn", i+1, heading(d), want)
		}
	}
}

// speedTicks spawns one entity carrying st at vel, runs n ticks at 60 TPS with the given Moving
// behaviors, and reports the Steering's base speed and the entity's Velocity.Value after each.
func speedTicks(t *testing.T, st steering.Steering, vel Velocity, moving []plugin.Rule, n int) (speeds, values []float64) {
	t.Helper()

	wm := testWorld()
	for _, b := range moving {
		if err := wm.movers.Add(b); err != nil {
			t.Fatal(err)
		}
	}
	wm.populate(testKind(
		Position{AABB: plane.NewAABB(geom.NewVec(500, 500), 10, 10)},
		vel,
		comp.Const(st),
	), []any{nil})

	var base goke.Comp[Base]
	var steer goke.Comp[steering.Steering]
	var query *goke.Query
	ecs := goke.New()
	ecs.Setup(append(wm.SetupSystems(), goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&base, &steer).Build()
	}})...)
	wm.RegSystems(ecs)
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { wm.RunPlan(rc, d); wm.clock.Replay(rc, d) })

	for range n {
		ecs.Tick(time.Second / 60)
		query.All()
		for query.Next() {
			cur := query.Cursor()
			for i := range cur.IDs {
				speeds = append(speeds, steer.Slice(cur)[i].Speed)
				values = append(values, base.Slice(cur)[i].Vel.Value)
			}
		}
	}
	return speeds, values
}

// halving is a Moving behavior that halves every entity's speed.
var halving = rule.On("halving", rule.All, func(m *rule.Moment[Moving]) rule.Step {
	return m.Call(func(_ plugin.Tick, m Moving) { m.Base.Vel.Value *= 0.5 })
})

func TestSteering_NoProfileLeavesSpeedAlone(t *testing.T) {
	_, values := speedTicks(t, steering.Steering{TurnRate: 0.5}, Velocity{Dir: east, Value: 60}, nil, 3)
	for tick, v := range values {
		if v != 60 {
			t.Fatalf("tick %d: Velocity.Value = %v, want the kind's 60 left alone without a profile", tick+1, v)
		}
	}
}

func TestSteering_SetsOffAtV0ThenAccelerates(t *testing.T) {
	st := steering.Steering{MaxSpeed: 100, Accel: 200, V0: 40, WantSpeed: 100}
	speeds, values := speedTicks(t, st, Velocity{Dir: east}, nil, 40)

	step := 200 * (time.Second / 60).Seconds() // one tick of Accel, at the tick length the harness uses
	if speeds[0] != 40 {
		t.Errorf("tick 1: Speed = %v, want V0 = 40 the instant it sets off", speeds[0])
	}
	if got, want := speeds[1], 40+step; math.Abs(got-want) > 1e-9 {
		t.Errorf("tick 2: Speed = %v, want %v (V0 plus one tick of Accel)", got, want)
	}
	for tick := 1; tick < len(speeds); tick++ {
		if speeds[tick] < speeds[tick-1] {
			t.Fatalf("tick %d: Speed fell from %v to %v while accelerating", tick+1, speeds[tick-1], speeds[tick])
		}
	}
	if last := speeds[len(speeds)-1]; last != 100 {
		t.Errorf("after 40 ticks Speed = %v, want it settled at MaxSpeed 100", last)
	}
	for tick := range speeds {
		if values[tick] != speeds[tick] {
			t.Fatalf("tick %d: Velocity.Value = %v, Speed = %v; the base speed is not written through", tick+1, values[tick], speeds[tick])
		}
	}
}

func TestSteering_SpeedIsHeldWithinTheProfile(t *testing.T) {
	s := &steering.Steering{MaxSpeed: 100}
	s.RequestSpeed(500)
	if s.WantSpeed != 100 {
		t.Errorf("RequestSpeed(500) asked for %v, want MaxSpeed 100", s.WantSpeed)
	}
	s.RequestSpeed(-1)
	if s.WantSpeed != 0 {
		t.Errorf("RequestSpeed(-1) asked for %v, want 0", s.WantSpeed)
	}
	none := &steering.Steering{}
	none.RequestSpeed(50)
	if none.WantSpeed != 0 {
		t.Errorf("RequestSpeed without a profile asked for %v, want nothing", none.WantSpeed)
	}
}

func TestSteering_BrakesToAHalt(t *testing.T) {
	st := steering.Steering{MaxSpeed: 100, Accel: 200, V0: 40, Speed: 100, WantSpeed: 0}
	speeds, _ := speedTicks(t, st, Velocity{Dir: east, Value: 100}, nil, 40)

	step := 200 * (time.Second / 60).Seconds()
	if got, want := speeds[0], 100-step; math.Abs(got-want) > 1e-9 {
		t.Errorf("tick 1: Speed = %v, want %v (one tick of braking)", got, want)
	}
	for tick := 1; tick < len(speeds); tick++ {
		if speeds[tick] > speeds[tick-1] || speeds[tick] < 0 {
			t.Fatalf("tick %d: Speed went from %v to %v while braking", tick+1, speeds[tick-1], speeds[tick])
		}
	}
	if last := speeds[len(speeds)-1]; last != 0 {
		t.Errorf("after 40 ticks Speed = %v, want a halt at 0", last)
	}
}

func TestSteering_NoAccelChangesSpeedAtOnce(t *testing.T) {
	st := steering.Steering{MaxSpeed: 100, WantSpeed: 70}
	speeds, _ := speedTicks(t, st, Velocity{Dir: east}, nil, 1)
	if speeds[0] != 70 {
		t.Errorf("tick 1: Speed = %v, want 70 at once with no Accel", speeds[0])
	}
}

func TestSteering_RewritesTheBaseSpeedAheadOfModifiers(t *testing.T) {
	st := steering.Steering{MaxSpeed: 100, WantSpeed: 100}
	speeds, values := speedTicks(t, st, Velocity{Dir: east}, []plugin.Rule{halving}, 3)
	for tick := range values {
		if got, want := values[tick], speeds[tick]*0.5; got != want {
			t.Fatalf("tick %d: Velocity.Value = %v, want %v — the modifier compounds instead of scaling a fresh base speed", tick+1, got, want)
		}
	}
}

func TestSteering_BrakesAtItsOwnRateWhenGivenOne(t *testing.T) {
	st := steering.Steering{MaxSpeed: 100, Accel: 200, Brake: 400, V0: 40, Speed: 100, WantSpeed: 0}
	speeds, _ := speedTicks(t, st, Velocity{Dir: east, Value: 100}, nil, 2)

	step := 400 * (time.Second / 60).Seconds()
	if got, want := speeds[0], 100-step; math.Abs(got-want) > 1e-9 {
		t.Errorf("tick 1: Speed = %v, want %v (one tick of Brake, not Accel)", got, want)
	}
	weak := steering.Steering{Accel: 200, Brake: 25}
	if weak.Braking() != 25 || (&steering.Steering{Accel: 200}).Braking() != 200 {
		t.Errorf("Braking = %v and %v, want Brake when set and Accel otherwise", weak.Braking(), (&steering.Steering{Accel: 200}).Braking())
	}
}
