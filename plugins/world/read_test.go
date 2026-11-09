package world_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// what a goroutine reads of the world: who stands in each box, who is under the effect, and
// whether the clock is in the phase
type worldReading struct {
	boxes [][]uid.UID64
	under []bool
	night bool
}

func readWorld(r *world.Read, ids []uid.UID64, lit effect.Effect,
	night tag.Tag[clock.Phase]) worldReading {
	var rd worldReading
	for y := float32(0); y < 400; y += 100 {
		for x := float32(0); x < 400; x += 100 {
			var in []uid.UID64
			r.Space().Query(geom.NewAABBAt(geom.NewVec(x, y), 100, 100), aabbworld.AnyCapability,
				func(id uid.UID64) { in = append(in, id) })
			slices.Sort(in)
			rd.boxes = append(rd.boxes, in)
		}
	}
	for _, id := range ids {
		rd.under = append(rd.under, r.Effects().Has(id, lit))
	}
	rd.night = r.Clock().In(night)
	return rd
}

func TestRead_TheWorkersReadTheWorldAtOnceAsTheGameDoes(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 10, MaxSize: 10},
	})
	night := w.Kinds().DefineTag[clock.Phase]("night")
	fx := w.Write().Effects()
	fx.Define("dusk", effect.Spec{effect.Lasts(time.Hour), effect.Grant(night)})
	fx.Define("lit", effect.Spec{effect.Lasts(time.Hour)})
	dusk, lit := fx.Named("dusk"), fx.Named("lit")
	kind.Define[world.Position](w.Kinds(), "dot", kind.Spec{
		comp.Load(func(p world.Position) world.Position { return p }),
		comp.Const(world.Velocity{}),
	})
	dot := kind.Named[world.Position](w.Kinds(), "dot")
	for i := range 64 {
		at := geom.NewVec(float32(i%8)*50+5, float32(i/8)*50+5)
		w.Write().Seed(dot.Entry(world.Position{AABB: plane.NewAABB(at, 10, 10)}))
	}
	pool := parallel.New(4)
	ctx := &installCtx{ecs: goke.New(), pool: pool}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Deliver(w.Kinds().Played()...); err != nil {
		t.Fatal(err)
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		w.Write().Clock().Replay(rc, d)
	})
	var ids []uid.UID64
	w.Read(0).Space().Query(geom.NewAABBAt(geom.NewVec(0, 0), 400, 400), aabbworld.AnyCapability,
		func(id uid.UID64) { ids = append(ids, id) })
	slices.Sort(ids)
	for i, id := range ids {
		if i%3 == 0 {
			fx.Cast(id, lit)
		}
	}
	fx.Cast(w.Read(0).Clock().Entity(), dusk)
	for range 3 {
		ctx.ecs.Tick(time.Second / 10)
	}
	got := make([]worldReading, 2*pool.Size())
	b := pool.Begin(func(wk, i int) {
		for range 20 {
			got[i] = readWorld(w.Read(wk), ids, lit, night)
		}
	}, nil)
	b.Add(len(got))
	b.Wait()
	want := readWorld(w.Read(0), ids, lit, night)
	if len(ids) != 64 || !want.night || !want.under[0] || want.under[1] {
		t.Fatalf("the game's goroutine reads %d entities, night %v, the first lit %v, the second "+
			"%v: the world proves nothing", len(ids), want.night, want.under[0], want.under[1])
	}
	for i := range got {
		if !slices.Equal(got[i].under, want.under) || got[i].night != want.night ||
			!slices.EqualFunc(got[i].boxes, want.boxes, slices.Equal) {
			t.Fatalf("task %d read %+v, the game's goroutine %+v", i, got[i], want)
		}
	}
}

func TestRead_SaysTheSectorOfAPointAndHowManyThereAre(t *testing.T) {
	cfg := world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 10, MaxSize: 10},
	}
	if r := world.NewPlugin(cfg).Read(0); r.SectorsLen() != 0 || r.SectorOf(300, 300) != 0 {
		t.Errorf("a world of no sectors has %d, the point (300, 300) in sector %d; want 0 and 0",
			r.SectorsLen(), r.SectorOf(300, 300))
	}
	// buckets of 64 a side, two of them a sector's: four sectors across, the last cut short
	cfg.Sectors = world.SectorsCfg{Buckets: 2}
	r := world.NewPlugin(cfg).Read(0)
	if n := r.SectorsLen(); n != 16 {
		t.Fatalf("the world is cut into %d sectors, want 4 by 4", n)
	}
	for _, c := range []struct {
		x, y float32
		want int
	}{{10, 10, 0}, {130, 10, 1}, {10, 130, 4}, {399, 399, 15}, {-5, 500, 12}} {
		if got := r.SectorOf(c.x, c.y); got != c.want {
			t.Errorf("the point (%v, %v) lies in sector %d, want %d", c.x, c.y, got, c.want)
		}
	}
}

func TestRead_SaysTheSectorsSide(t *testing.T) {
	cfg := world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 10, MaxSize: 10},
	}
	if s := world.NewPlugin(cfg).Read(0).SectorSide(); s != 0 {
		t.Errorf("a world of no sectors says its sector's side is %v, want 0", s)
	}
	// buckets of 64 a side, two of them a sector's
	cfg.Sectors = world.SectorsCfg{Buckets: 2}
	if s := world.NewPlugin(cfg).Read(0).SectorSide(); s != 128 {
		t.Errorf("the sector's side is %v, want 128", s)
	}
}

func TestRead_SmallIsAQuarterOfASectorsSideUnlessSet(t *testing.T) {
	cfg := world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 10, MaxSize: 10},
	}
	if s := world.NewPlugin(cfg).Read(0).Small(); s != 0 {
		t.Errorf("a world of no sectors says small is %v, want 0", s)
	}
	// sectors 128 a side
	cfg.Sectors = world.SectorsCfg{Buckets: 2}
	if s := world.NewPlugin(cfg).Read(0).Small(); s != 32 {
		t.Errorf("small unset is %v, want a quarter of a sector's side, 32", s)
	}
	cfg.Sectors.Small = 50
	if s := world.NewPlugin(cfg).Read(0).Small(); s != 50 {
		t.Errorf("small set to 50 is %v", s)
	}
}
