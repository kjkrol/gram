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
