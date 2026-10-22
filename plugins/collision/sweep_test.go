package collision_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type flight struct {
	Dir     geom.Vec
	Step    float64
	Through bool
}

type shooter struct{}

type piece struct {
	x, y, size float64
	fly        *flight
	mass       float64 // 0: no Physics
	shooter    bool
}

type flyer struct {
	q     *goke.Query
	base  goke.Comp[world.Base]
	sweep goke.Comp[collision.Sweep]
	fl    goke.Comp[flight]

	shooters *goke.Query
	sbase    goke.Comp[world.Base]
}

func (f *flyer) Init(si *goke.SysInit) {
	f.q = si.NewQueryBuilder(&f.base, &f.sweep, &f.fl).Build()
	f.shooters = si.NewQueryBuilder(&f.sbase).Include(goke.Include[shooter]()).Build()
}

func (f *flyer) Update(*goke.CmdBuf, time.Duration) {
	var who uid.UID64
	var any bool
	for f.shooters.All(); f.shooters.Next(); {
		if ids := f.shooters.Cursor().IDs; len(ids) > 0 {
			who, any = ids[0], true
		}
	}
	for f.q.All(); f.q.Next(); {
		cur := f.q.Cursor()
		bases, sweeps, flights := f.base.Slice(cur), f.sweep.Slice(cur), f.fl.Slice(cur)
		for i := range cur.IDs {
			if flights[i].Through && any {
				sweeps[i].Ignore, sweeps[i].Ignoring = who, true
			}
			pos := &bases[i].Pos
			sweeps[i].From = pos.Center()
			to := geom.NewVec(pos.TopLeft.X+flights[i].Dir.X*flights[i].Step, pos.TopLeft.Y+flights[i].Dir.Y*flights[i].Step)
			pos.AABB = plane.NewAABB(to, pos.Size.X, pos.Size.Y)
		}
	}
}

type outcome struct {
	id       uid.UID64
	base     world.Base
	contacts []collision.Contact
}

func sweptRun(t *testing.T, edges aabbworld.Edges, field collision.Field, ticks int, bodies ...piece) ([]outcome, *collision.ContactStats) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000, Edges: edges},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 4, MaxSize: 40},
	})
	var stats collision.ContactStats
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m).WithStats(&stats)
	if field != nil {
		c.WithField(field)
	}
	kinds := make([]kind.Of[piece], len(bodies))
	for i, b := range bodies {
		spec := kind.Spec{
			comp.Load(func(b piece) world.Position { return posAt(b.x, b.y, b.size, b.size) }),
			comp.Const(world.Velocity{}),
			comp.Const(collision.Collider{}),
		}
		if b.fly != nil {
			spec = append(spec,
				comp.Load(func(b piece) collision.Sweep { return collision.Sweep{From: geom.NewVec(b.x+b.size/2, b.y+b.size/2)} }),
				comp.Load(func(b piece) flight { return *b.fly }))
		}
		if b.mass != 0 {
			spec = append(spec, comp.Load(func(b piece) collision.Physics { return collision.Physics{Mass: b.mass} }))
		}
		if b.shooter {
			spec = append(spec, comp.Const(shooter{}))
		}
		kind.Define[piece](w.Kinds(), "piece"+string(rune('a'+i)), spec)
		kinds[i] = kind.Named[piece](w.Kinds(), "piece"+string(rune('a'+i)))
		w.Write().Seed(kinds[i].Entry(b))
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}

	ctx := collisiontest.NewInstallCtx(goke.New())
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[world.Base]
	var coll goke.Comp[collision.Collider]
	var q *goke.Query
	ecs := ctx.ECS()
	ecs.Setup(append(ctx.Systems(), goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base, &coll).Build() }})...)
	fly := ecs.RegSys(&flyer{})
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(fly, d)
		rc.Sync()
		w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		c.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	for range ticks {
		ecs.Tick(time.Second / 60)
	}

	out := make([]outcome, len(bodies))
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i, id := range cur.IDs {
			b := base.Slice(cur)[i]
			for k := range kinds {
				if kinds[k].ID() == b.TypeID {
					out[k] = outcome{id: id, base: b, contacts: append([]collision.Contact(nil), coll.Slice(cur)[i].Contacts()...)}
				}
			}
		}
	}
	return out, &stats
}

var east = geom.NewVec(1, 0)

func TestSweep_AFastShotStrikesWhatLiesOnItsPath(t *testing.T) {
	out, stats := sweptRun(t, 0, nil, 1,
		piece{x: 100, y: 500, size: 4, fly: &flight{Dir: east, Step: 60}},
		piece{x: 130, y: 498, size: 10, mass: 1},
	)
	shot, target := out[0], out[1]
	if len(shot.contacts) != 1 || len(target.contacts) != 1 {
		t.Fatalf("the shot has %d contacts and the target %d, want one each", len(shot.contacts), len(target.contacts))
	}
	hit := shot.contacts[0]
	if hit.Other != target.id || !hit.Sensed || math.Abs(hit.Along-26.0/60) > 1e-9 || hit.Normal != geom.NewVec(-1, 0) {
		t.Errorf("the shot's contact is %+v, want the target, sensed, 26/60 along, the way back west", hit)
	}
	if got := target.contacts[0]; got.Other != shot.id || !got.Sensed || math.Abs(got.Along-26.0/60) > 1e-9 || got.Normal != geom.NewVec(1, 0) {
		t.Errorf("the target's contact is %+v, want the shot, sensed, 26/60 along, the way east", got)
	}
	if target.base.Pos.TopLeft.X != 130 {
		t.Errorf("the target was pushed to %v, want left where it stood", target.base.Pos.TopLeft.X)
	}
	if shot.base.Pos.TopLeft.X != 160 {
		t.Errorf("the shot's box starts at %v, want 160 where its step ended", shot.base.Pos.TopLeft.X)
	}
	if stats.Counter != 1 {
		t.Errorf("%d contacts counted, want 1", stats.Counter)
	}
}

func TestSweep_TheSpaceHoldsOnlyTheRealBoxAfterTheTick(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 4, MaxSize: 4},
	})
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	kind.Define[piece](w.Kinds(), "shot", kind.Spec{
		comp.Load(func(b piece) world.Position { return posAt(b.x, b.y, b.size, b.size) }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
		comp.Load(func(b piece) collision.Sweep { return collision.Sweep{From: geom.NewVec(b.x+2, b.y+2)} }),
		comp.Const(flight{Dir: east, Step: 60}),
	})
	shots := kind.Named[piece](w.Kinds(), "shot")
	w.Write().Seed(shots.Entry(piece{x: 100, y: 500, size: 4}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := collisiontest.NewInstallCtx(goke.New())
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatal(err)
	}
	ecs := ctx.ECS()
	ecs.Setup(ctx.Systems()...)
	fly := ecs.RegSys(&flyer{})
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(fly, d)
		rc.Sync()
		w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		c.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	ecs.Tick(time.Second / 60)
	count := func(box geom.AABB) int {
		return w.Read(0).Space().Query(box, aabbworld.AnyCapability, func(uid.UID64) {})
	}
	if n := count(geom.NewAABBAt(geom.NewVec(110, 495), 40, 10)); n != 0 {
		t.Errorf("the space finds %d pieces on the shot's path after the tick, want none", n)
	}
	if n := count(geom.NewAABBAt(geom.NewVec(158, 495), 10, 10)); n != 1 {
		t.Errorf("the space finds %d pieces where the shot ended, want it", n)
	}
}

func TestSweep_OnlyTheNearestContactStays(t *testing.T) {
	out, stats := sweptRun(t, 0, nil, 1,
		piece{x: 100, y: 500, size: 4, fly: &flight{Dir: east, Step: 60}},
		piece{x: 145, y: 498, size: 10, mass: 1},
		piece{x: 120, y: 498, size: 10, mass: 1},
	)
	shot, far, near := out[0], out[1], out[2]
	if len(shot.contacts) != 1 || shot.contacts[0].Other != near.id {
		t.Errorf("the shot's contacts are %+v, want the nearer box alone", shot.contacts)
	}
	if len(near.contacts) != 1 || len(far.contacts) != 0 {
		t.Errorf("the near box has %d contacts and the far one %d, want one and none", len(near.contacts), len(far.contacts))
	}
	if stats.Counter != 1 {
		t.Errorf("%d contacts counted, want 1", stats.Counter)
	}
}

func TestSweep_ASlantingPathMissesWhatLiesBesideIt(t *testing.T) {
	out, _ := sweptRun(t, 0, nil, 1,
		piece{x: 100, y: 500, size: 4, fly: &flight{Dir: geom.NewVec(1, 1), Step: 60}},
		piece{x: 150, y: 505, size: 10, mass: 1},
	)
	if len(out[0].contacts) != 0 || len(out[1].contacts) != 0 {
		t.Errorf("the shot has %d contacts and the box %d, want none: the box is beside the path", len(out[0].contacts), len(out[1].contacts))
	}
}

func TestSweep_TwoSweptPassThroughEachOther(t *testing.T) {
	out, _ := sweptRun(t, 0, nil, 1,
		piece{x: 100, y: 500, size: 4, fly: &flight{Dir: east, Step: 60}},
		piece{x: 90, y: 500, size: 4, fly: &flight{Dir: east, Step: 80}},
	)
	if len(out[0].contacts) != 0 || len(out[1].contacts) != 0 {
		t.Errorf("the shots have %d and %d contacts, want none", len(out[0].contacts), len(out[1].contacts))
	}
}

func TestSweep_IgnoresWhatItIsToldTo(t *testing.T) {
	out, _ := sweptRun(t, 0, nil, 1,
		piece{x: 100, y: 500, size: 4, fly: &flight{Dir: east, Step: 60, Through: true}},
		piece{x: 92, y: 492, size: 20, mass: 1, shooter: true},
		piece{x: 130, y: 498, size: 10, mass: 1},
	)
	shot, who, target := out[0], out[1], out[2]
	if len(shot.contacts) != 1 || shot.contacts[0].Other != target.id {
		t.Errorf("the shot's contacts are %+v, want the target alone", shot.contacts)
	}
	if len(who.contacts) != 0 {
		t.Errorf("the shooter has %d contacts, want none", len(who.contacts))
	}
}

type boxesField struct {
	boxes []collision.FieldBox
	walls []collide.FieldWall
}

func (f *boxesField) Solid(_ world.Layers, _ collision.Band, box geom.AABB, visit func(collision.FieldBox) bool) {
	for _, fb := range f.boxes {
		if box.Intersects(fb.Box) && !visit(fb) {
			return
		}
	}
}

func (f *boxesField) Overhang(world.Layers, geom.AABB) float64 { return 0 }
func (f *boxesField) Walls(_ world.Layers, _ collision.Band, box geom.AABB,
	visit func(collide.FieldWall) bool) {
	for _, w := range f.walls {
		lo := geom.NewVec(min(w.A.X, w.B.X)-w.Half, min(w.A.Y, w.B.Y)-w.Half)
		hi := geom.NewVec(max(w.A.X, w.B.X)+w.Half, max(w.A.Y, w.B.Y)+w.Half)
		if box.Intersects(geom.NewAABB(lo, hi)) && !visit(w) {
			return
		}
	}
}

const allSides = collide.Left | collide.Right | collide.Top | collide.Bottom

func TestSweep_TheGroundNearerThanTheBoxWins(t *testing.T) {
	wall := &boxesField{boxes: []collision.FieldBox{{Box: geom.NewAABBAt(geom.NewVec(120, 0), 32, 1000), Cell: 7, Open: allSides}}}
	out, stats := sweptRun(t, 0, wall, 1,
		piece{x: 100, y: 500, size: 4, fly: &flight{Dir: east, Step: 60}},
		piece{x: 160, y: 498, size: 10, mass: 1},
		piece{x: 118, y: 600, size: 10, mass: 1},
	)
	shot, box, walker := out[0], out[1], out[2]
	if len(shot.contacts) != 1 {
		t.Fatalf("the shot has %d contacts, want the wall's alone", len(shot.contacts))
	}
	hit := shot.contacts[0]
	if !hit.Terrain || hit.Cell != 7 || !hit.Sensed || math.Abs(hit.Along-16.0/60) > 1e-9 || hit.Normal != geom.NewVec(-1, 0) {
		t.Errorf("the shot's contact is %+v, want the wall, sensed, 16/60 along, the way back west", hit)
	}
	if len(box.contacts) != 0 {
		t.Errorf("the box beyond the wall has %d contacts, want none", len(box.contacts))
	}
	if len(walker.contacts) != 1 || walker.contacts[0].Along != 1 || walker.contacts[0].Sensed {
		t.Errorf("the walker's contacts are %+v, want the wall's at the end of its step, not sensed", walker.contacts)
	}
	if stats.Counter != 0 {
		t.Errorf("%d pairs counted, want none: the ground is no pair", stats.Counter)
	}
}

func TestSweep_ACellOfSeveralBoxesIsStruckWhereTheSegmentCrossesIt(t *testing.T) {
	cell := &boxesField{boxes: []collision.FieldBox{
		// the first in the stretch but off the segment, the second on it
		{Box: geom.NewAABBAt(geom.NewVec(130, 500), 18, 10), Cell: 7, Open: allSides},
		{Box: geom.NewAABBAt(geom.NewVec(120, 520), 10, 10), Cell: 7, Open: allSides},
	}}
	slant := geom.NewVec(math.Sqrt2/2, math.Sqrt2/2)
	out, _ := sweptRun(t, 0, cell, 1,
		piece{x: 100, y: 500, size: 4, fly: &flight{Dir: slant, Step: 60}},
	)
	got := out[0].contacts
	if len(got) != 1 || !got[0].Terrain || got[0].Cell != 7 {
		t.Fatalf("the shot's contacts are %+v, want the cell struck once", got)
	}
	if want := 16 * math.Sqrt2 / 60; math.Abs(got[0].Along-want) > 1e-9 {
		t.Errorf("struck %v along the step, want %v: where the segment reaches the box on it", got[0].Along, want)
	}
}

func TestSweep_ACellOfSeveralBoxesIsStruckAtItsNearest(t *testing.T) {
	cell := &boxesField{boxes: []collision.FieldBox{
		// the middle handed first, the cap nearer
		{Box: geom.NewAABBAt(geom.NewVec(130, 490), 20, 30), Cell: 7, Open: allSides},
		{Box: geom.NewAABBAt(geom.NewVec(120, 490), 10, 30), Cell: 7, Open: allSides},
	}}
	out, stats := sweptRun(t, 0, cell, 1,
		piece{x: 100, y: 500, size: 4, fly: &flight{Dir: east, Step: 60}},
		piece{x: 122, y: 498, size: 10, mass: 1}, // behind the cap, before the middle
	)
	shot, box := out[0], out[1]
	if len(shot.contacts) != 1 || !shot.contacts[0].Terrain || shot.contacts[0].Cell != 7 {
		t.Fatalf("the shot's contacts are %+v, want the cell alone", shot.contacts)
	}
	if hit := shot.contacts[0]; math.Abs(hit.Along-16.0/60) > 1e-9 || hit.Normal != geom.NewVec(-1, 0) {
		t.Errorf("struck %v along the step, the way out %v; want 16/60 at the cap, the way back west", hit.Along, hit.Normal)
	}
	for _, c := range box.contacts {
		if !c.Terrain && c.Other == shot.id {
			t.Errorf("the box behind the cap was struck by the shot: %+v, want the wall to take it", c)
		}
	}
	if stats.Counter != 0 {
		t.Errorf("%d pairs counted, want none", stats.Counter)
	}
}

func TestSweep_AnImmovableSweptEntityIsASensor(t *testing.T) {
	wall := &boxesField{boxes: []collision.FieldBox{{Box: geom.NewAABBAt(geom.NewVec(150, 0), 32, 1000), Cell: 7, Open: allSides}}}
	out, _ := sweptRun(t, 0, wall, 1,
		piece{x: 100, y: 500, size: 4, fly: &flight{Dir: east, Step: 60}, mass: math.Inf(1)},
		piece{x: 120, y: 498, size: 10, mass: 1},
	)
	shot, box := out[0], out[1]
	if box.base.Pos.TopLeft.X != 120 || box.base.Vel.Value != 0 {
		t.Errorf("the box ended at %v moving %v, want where it stood, still", box.base.Pos.TopLeft.X, box.base.Vel.Value)
	}
	if len(shot.contacts) != 1 || !shot.contacts[0].Sensed || shot.contacts[0].Impact != 0 {
		t.Errorf("the shot's contacts are %+v, want the box alone, sensed, no impact", shot.contacts)
	}
	out, _ = sweptRun(t, 0, wall, 1,
		piece{x: 100, y: 500, size: 4, fly: &flight{Dir: east, Step: 60}, mass: math.Inf(1)},
	)
	if got := out[0].contacts; len(got) != 1 || !got[0].Terrain {
		t.Errorf("the shot's contacts are %+v, want the wall's", got)
	}
}

func TestSweep_AWrappingWorldRefusesIt(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a swept entity in a wrapping world went unrefused")
		}
	}()
	sweptRun(t, aabbworld.Torus, nil, 1, piece{x: 100, y: 500, size: 4, fly: &flight{Dir: east, Step: 60}})
}

func TestSweep_AShotStrikesASlantedWall(t *testing.T) {
	wall := &boxesField{walls: []collide.FieldWall{
		{A: geom.NewVec(100, 450), B: geom.NewVec(200, 550), Half: 1, Cell: 9},
	}}
	out, _ := sweptRun(t, 0, wall, 1,
		piece{x: 100, y: 498, size: 4, fly: &flight{Dir: east, Step: 80}})
	if len(out[0].contacts) != 1 {
		t.Fatalf("the shot has %d contacts, want the wall's", len(out[0].contacts))
	}
	hit := out[0].contacts[0]
	r := math.Sqrt2 / 2
	if !hit.Terrain || hit.Cell != 9 || math.Abs(hit.Normal.X+r) > 1e-9 ||
		math.Abs(hit.Normal.Y-r) > 1e-9 || hit.Along <= 0 || hit.Along >= 1 {
		t.Errorf("the shot's contact is %+v, want the wall's, on the way, square to it", hit)
	}
}
