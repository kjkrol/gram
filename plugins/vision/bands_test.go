package vision_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

type anchor struct{ x, y float32 }

func (a *anchor) Anchors(at func(x, y float32)) { at(a.x, a.y) }

var _ sector.Anchors = (*anchor)(nil)

func TestScan_FarOffAnObserverSeesNothing(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4096, Height: 512},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 20},
		Sectors: world.SectorsCfg{
			Buckets: 2,
			Bands: []sector.Band{
				{Reach: 600},
				{Reach: 1200, Pace: 4},
			},
		},
	})
	eyed := &anchor{x: 1200, y: 256}
	w.Write().Anchor(eyed)
	v := vision.NewPlugin(w)
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := v.Install(ctx); err != nil {
		t.Fatal(err)
	}
	kind.Define[spawn](w.Kinds(), "observer", kind.Spec{
		comp.Load(at),
		comp.Const(world.Velocity{}),
		comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: 300}),
		comp.Const(world.Eye{Angle: math.Pi / 2}),
		comp.Const(vision.Sighted{}),
	})
	kind.Define[spawn](w.Kinds(), "target", kind.Spec{comp.Load(at), comp.Const(world.Velocity{})})
	w.Write().Seed(kind.Named[spawn](w.Kinds(), "observer").Entry(spawn{x: 1200, y: 250}),
		kind.Named[spawn](w.Kinds(), "target").Entry(spawn{x: 1300, y: 250}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var sighted goke.Comp[vision.Sighted]
	var query *goke.Query
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&sighted).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		v.RunPlan(rc, d)
		w.Write().Clock().Replay(rc, d)
	})
	seen := func() uint8 {
		for query.All(); query.Next(); {
			return sighted.Slice(query.Cursor())[0].Count
		}
		return 0
	}
	for range 3 {
		ctx.ecs.Tick(time.Second / 60)
	}
	if seen() == 0 {
		t.Fatal("near, the observer sees nothing")
	}
	eyed.x = 100
	for range 3 {
		ctx.ecs.Tick(time.Second / 60)
	}
	if n := seen(); n != 0 {
		t.Errorf("in the second band the observer sees %d, want nothing", n)
	}
}
