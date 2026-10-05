package vision_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// installCtx is the plugin.Installer a Stage would hand over, minus the engine.
type installCtx struct {
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
	ecs     *goke.ECS
	pending []func() []goke.System
	tracked []any
}

func (c *installCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.tracked = append(c.tracked, m)
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.tracked = append(c.tracked, p)
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installCtx) ECS() *goke.ECS                                  { return c.ecs }

// spawn describes one entity the fixture puts in the world: a 10×10 box unless size says
// otherwise, cutting sight unless tau says how see-through it is, on every plane unless layers
// says which.
type spawn struct {
	x, y    float64
	size    float64
	tau     float64
	layers  world.Layers
	z       *world.Z // heights, in a scene with heights
	sight   *look    // nil for something that is merely seen
	outline bool
	heading geom.Vec // the way it moves; zero, none
	bare    bool     // a sight without a Sighted, which vision gives it
}

// look is a Sight with the Eye it sees from: how wide, and how high in a scene with heights.
type look struct {
	vision.Sight
	world.Eye
}

// relief is what a scene with heights stands on: nil for flat ground at 0.
type relief struct {
	ground ground.Heights
	step   float64
	scale  world.Scale
}

func at(d spawn) world.Position {
	size := d.size
	if size == 0 {
		size = 10
	}
	return world.Position{AABB: plane.NewAABB(geom.NewVec(d.x, d.y), size, size)}
}

// scene installs world+vision on a flat world, spawns everything, ticks once; returns observers and
// what they saw.
func scene(t *testing.T, spawns ...spawn) ([]uid.UID64, []vision.Sighted, []vision.SightOutline) {
	t.Helper()
	return sceneIn(t, nil, spawns...)
}

// sceneIn is scene in a world with heights standing on r (nil: a flat world).
func sceneIn(t *testing.T, r *relief, spawns ...spawn) ([]uid.UID64, []vision.Sighted, []vision.SightOutline) {
	t.Helper()
	return sceneWith(t, r, 0, spawns...)
}

// sceneWith is sceneIn with the scan shared among workers goroutines at most (0: the CPUs, 1: none).
func sceneWith(t *testing.T, r *relief, workers int, spawns ...spawn) ([]uid.UID64, []vision.Sighted, []vision.SightOutline) {
	t.Helper()

	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 2000, Height: 2000},
		Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 1, MaxSize: 100},
		Heights:  r != nil,
		Scale:    scaleOf(r),
	})
	v := vision.NewPlugin(w).WithWorkers(workers)
	if r != nil {
		heights := r.ground
		v.WithHeights(func() ground.Heights { return heights }).WithGroundStep(r.step)
	}

	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatalf("world Install: %v", err)
	}
	if err := v.Install(ctx); err != nil {
		t.Fatalf("vision Install: %v", err)
	}

	for i, s := range spawns {
		spec := kind.Spec{
			comp.Load(at),
			comp.Load(func(s spawn) world.Velocity { return world.Velocity{Dir: s.heading} }),
		}
		if s.tau > 0 {
			spec = append(spec, comp.Const(vision.Transparency{Value: s.tau}))
		}
		if s.layers != 0 {
			spec = append(spec, comp.Const(s.layers))
		}
		if s.z != nil {
			spec = append(spec, comp.Const(*s.z))
		}
		if s.sight != nil {
			spec = append(spec, comp.Const(s.sight.Sight), comp.Const(s.sight.Eye))
			if !s.bare {
				spec = append(spec, comp.Const(vision.Sighted{}))
			}
			if s.outline {
				spec = append(spec, comp.Const(vision.SightOutline{}))
			}
		}
		kind.Define[spawn](w.Kinds(), kindName(i), spec)
		w.Seed(kind.Named[spawn](w.Kinds(), kindName(i)).Entry(s))
	}
	if err := w.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	var sightComp goke.Comp[vision.Sighted]
	var outlineComp goke.OptComp[vision.SightOutline]
	var query *goke.Query
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&sightComp).Optional(&outlineComp).Build()
	}})
	ctx.ecs.Setup(systems...)

	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		v.RunPlan(rc, d)
		w.Clock().Replay(rc, d)
	})
	ctx.ecs.Tick(time.Second / 60)

	var ids []uid.UID64
	var seen []vision.Sighted
	var outlines []vision.SightOutline
	query.All()
	for query.Next() {
		cursor := query.Cursor()
		got := sightComp.Slice(cursor)
		var outs []vision.SightOutline
		if outlineComp.Present(cursor) {
			outs = outlineComp.Slice(cursor)
		}
		for i, id := range cursor.IDs {
			ids = append(ids, id)
			seen = append(seen, got[i])
			if outs != nil {
				outlines = append(outlines, outs[i])
			}
		}
	}
	return ids, seen, outlines
}

func kindName(i int) string { return string(rune('a' + i)) }

func eastward(half, radius float64) *look {
	return &look{Sight: vision.Sight{Facing: geom.NewVec(1.0, 0.0), Radius: radius}, Eye: world.Eye{Angle: 2 * half}}
}

func TestScan_ReportsWhatIsInTheConeNearestFirst(t *testing.T) {
	_, seen, _ := scene(t,
		spawn{x: 500, y: 500, sight: eastward(math.Pi/4, 600)},
		spawn{x: 900, y: 560},
		spawn{x: 700, y: 500},
	)

	if len(seen) != 1 {
		t.Fatalf("%d entities carry Sight, want 1", len(seen))
	}
	if seen[0].Count != 2 {
		t.Fatalf("saw %d entities, want 2", seen[0].Count)
	}
	if seen[0].Dists[0] >= seen[0].Dists[1] {
		t.Errorf("distances %v, %v are not nearest-first", seen[0].Dists[0], seen[0].Dists[1])
	}
}

// A Sight Ahead looks the way its entity moves, its Facing only while it has no heading.
func TestScan_ASightAheadLooksTheWayItMoves(t *testing.T) {
	ahead := eastward(math.Pi/8, 600)
	ahead.Ahead = true
	north := geom.NewVec(0, 1)
	for _, c := range []struct {
		heading geom.Vec
		want    float32 // how far off the one it sees is
	}{{north, 500}, {geom.Vec{}, 300}} {
		_, seen, _ := scene(t,
			spawn{x: 500, y: 500, sight: ahead, heading: c.heading},
			spawn{x: 800, y: 500},  // east of it, 300 off
			spawn{x: 500, y: 1000}, // north of it, 500 off
		)
		if seen[0].Count != 1 || math.Abs(float64(seen[0].Dists[0]-c.want)) > 15 {
			t.Errorf("heading %v: saw %d, the nearest %v off; want one, %v off", c.heading, seen[0].Count, seen[0].Dists[0], c.want)
		}
	}
}

// An observer without a Sighted is given one at its first step and scanned from the next.
func TestScan_GivesASightedWhereThereIsNone(t *testing.T) {
	ids, seen, _ := scene(t,
		spawn{x: 500, y: 500, sight: eastward(math.Pi/4, 600), bare: true},
		spawn{x: 700, y: 500},
	)
	if len(ids) != 1 || seen[0].Count != 0 {
		t.Errorf("after the first step: %d observers with a Sighted, the first seeing %v; want one given, seeing nothing yet", len(ids), seen)
	}
}

func TestScan_IgnoresWhatFallsOutsideTheCone(t *testing.T) {
	_, seen, _ := scene(t,
		spawn{x: 500, y: 500, sight: eastward(math.Pi/8, 600)},
		spawn{x: 700, y: 500},
		spawn{x: 500, y: 900},
		spawn{x: 200, y: 500},
	)

	if seen[0].Count != 1 {
		t.Errorf("saw %d entities, want only the one ahead", seen[0].Count)
	}
}

func TestScan_KeepsAtMostMaxSeen(t *testing.T) {
	spawns := []spawn{{x: 200, y: 500, sight: eastward(math.Pi/3, 900)}}
	for i := range vision.MaxSeen + 4 {
		spawns = append(spawns, spawn{x: float64(400 + i*40), y: float64(480 + i*8)})
	}

	_, seen, _ := scene(t, spawns...)
	if int(seen[0].Count) > vision.MaxSeen {
		t.Errorf("recorded %d sightings, want at most MaxSeen (%d)", seen[0].Count, vision.MaxSeen)
	}
	if seen[0].Count == 0 {
		t.Fatal("recorded nothing, so the cap was never under pressure")
	}
}

func TestScan_WorksWithoutAnOutline(t *testing.T) {
	_, seen, outlines := scene(t,
		spawn{x: 500, y: 500, sight: eastward(math.Pi/4, 600)},
		spawn{x: 700, y: 500},
	)

	if len(outlines) != 0 {
		t.Errorf("%d outlines computed, want none", len(outlines))
	}
	if seen[0].Count != 1 {
		t.Errorf("saw %d entities without an outline, want 1", seen[0].Count)
	}
}

func TestScan_FillsTheOutlineWhenAsked(t *testing.T) {
	_, _, outlines := scene(t,
		spawn{x: 500, y: 500, sight: eastward(math.Pi/6, 300), outline: true},
		spawn{x: 700, y: 500},
	)

	if len(outlines) != 1 {
		t.Fatalf("%d outlines, want 1", len(outlines))
	}
	o := outlines[0]
	if o.Count < 2 {
		t.Fatalf("outline has %d samples, want at least the two cone edges", o.Count)
	}
	blocked := false
	for i := range int(o.Count) {
		if o.Depths[i] <= 0 || o.Depths[i] > 300 {
			t.Fatalf("sample %d reaches %v, outside (0,300]", i, o.Depths[i])
		}
		blocked = blocked || o.Depths[i] < 300
	}
	if !blocked {
		t.Error("nothing shortened the outline, though something stands straight ahead")
	}
}

// A cone wider and longer than the buffer was sized for must still fit it.
func TestScan_OutlineNeverOverrunsItsBuffer(t *testing.T) {
	huge := eastward(math.Pi/2-0.01, 1900)
	_, _, outlines := scene(t,
		spawn{x: 50, y: 500, sight: huge, outline: true},
		spawn{x: 700, y: 500},
	)

	if int(outlines[0].Count) > vision.MaxSamples {
		t.Errorf("outline holds %d samples, want at most MaxSamples (%d)", outlines[0].Count, vision.MaxSamples)
	}
}

func TestMaxSamples_IsTheSmallestThatHoldsTheTolerance(t *testing.T) {
	half := float64(vision.MaxHalfAngleMilli) / 1000
	arc := vision.MaxSightRadius * 2 * half

	if drift := arc / float64(vision.MaxSamples-1); drift > vision.EdgeTolerance {
		t.Errorf("a full-size cone drifts %.3f units at full range, over EdgeTolerance (%d)", drift, vision.EdgeTolerance)
	}
	if drift := arc / float64(vision.MaxSamples-2); drift <= vision.EdgeTolerance {
		t.Errorf("one sample fewer still drifts only %.3f — MaxSamples (%d) is bigger than it needs to be", drift, vision.MaxSamples)
	}
}

func TestScan_ClearsSightedWhenTheConeIsUnanswerable(t *testing.T) {
	blind := eastward(0, 0)
	_, seen, _ := scene(t,
		spawn{x: 500, y: 500, sight: blind},
		spawn{x: 700, y: 500},
	)

	if seen[0].Count != 0 {
		t.Errorf("a blind entity recorded %d sightings, want none", seen[0].Count)
	}
}

// scaleOf is r's scale, none without a relief.
func scaleOf(r *relief) world.Scale {
	if r == nil {
		return world.Scale{}
	}
	return r.scale
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *installCtx) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *installCtx) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
