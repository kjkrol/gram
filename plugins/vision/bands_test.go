package vision_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// two observers looking at (x, y) over a world w by h: the one a pixel a world unit, the other a
// quarter of a pixel seeing four times as far; under a ladder seeing from half a pixel a world
// unit, L2 within its sight of the point, within four times that the level its lower thresholds
// give a quarter of a pixel, L6 beyond
type observers [2]camera.Camera

var _ sector.Observers = (*observers)(nil)

func newObservers(w, h uint32, x, y float32) *observers {
	o := &observers{cameratest.NewFromSpace(w, h, 0), cameratest.NewFromSpace(w, h, 0)}
	for _, cam := range o {
		cam.SetViewport(2, 2)
	}
	o[1].ZoomOut(4, 0, 0)
	o[1].(interface{ See(factor float32) }).See(4)
	o.look(x, y)
	return o
}

// has both observers look at (x, y)
func (o *observers) look(x, y float32) {
	for _, cam := range o {
		cam.CenterOn(x, y, 0)
	}
}

func (o *observers) Observers(each func(cam camera.Camera)) {
	for _, cam := range o {
		each(cam)
	}
}

// far off is four sectors on from where the cameras look away: room for a sector a level coarser
// each between
func TestScan_AnObserverSeesEveryStepThroughL4AndNothingFromL5(t *testing.T) {
	for _, c := range []struct {
		level  lod.Level
		scales [lod.Levels - 1]float32
		sight  float32
		sees   bool
	}{
		{lod.L3, [lod.Levels - 1]float32{0, 0, 0.5, 0.2}, 600, true},
		{lod.L4, [lod.Levels - 1]float32{0, 0, 0.5, 0.4, 0.2}, 600, true},
		{lod.L5, [lod.Levels - 1]float32{0, 0, 0.5, 0.4, 0.3}, 600, false},
		{lod.L6, [lod.Levels - 1]float32{0, 0, 0.5, 0.4, 0.3}, 400, false},
	} {
		t.Run(fmt.Sprintf("L%d", c.level), func(t *testing.T) {
			sees, level := lookedAway(t, c.scales, c.sight)
			if level != c.level {
				t.Fatalf("looked away from, the observer is at L%d, want L%d", level, c.level)
			}
			if sees != c.sees {
				t.Errorf("looked away from, the observer sees %v, want %v", sees, c.sees)
			}
		})
	}
}

// whether an observer seeing a target at (2200, 250) still sees it once the cameras look at
// (100, 256), under a ladder of scales seeing sight, and the level it is at then
func lookedAway(t *testing.T, scales [lod.Levels - 1]float32, sight float32) (bool, lod.Level) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4096, Height: 512},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 20},
		Sectors:  world.SectorsCfg{Buckets: 2},
		LOD:      lod.Ladder{Scales: scales, Sight: sight},
	})
	eyed := newObservers(4096, 512, 2200, 256)
	w.Write().Observe(eyed)
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
		comp.Const(vision.Watched{}),
	})
	kind.Define[spawn](w.Kinds(), "target", kind.Spec{comp.Load(at), comp.Const(world.Velocity{})})
	w.Write().Seed(kind.Named[spawn](w.Kinds(), "observer").Entry(spawn{x: 2200, y: 250}),
		kind.Named[spawn](w.Kinds(), "target").Entry(spawn{x: 2300, y: 250}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var sighted goke.Comp[vision.Sighted]
	var l3 goke.OptComp[sector.L3]
	var l4 goke.OptComp[sector.L4]
	var l5 goke.OptComp[sector.L5]
	var l6 goke.OptComp[sector.L6]
	var query *goke.Query
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&sighted).Optional(&l3, &l4, &l5, &l6).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		v.RunPlan(rc, d)
		w.Write().Clock().Replay(rc, d)
	})
	seen := func() (uint8, lod.Level) {
		for query.All(); query.Next(); {
			cur, level := query.Cursor(), lod.L2
			for k, mark := range []bool{l3.Present(cur), l4.Present(cur), l5.Present(cur),
				l6.Present(cur)} {
				if mark {
					level = lod.L3 + lod.Level(k)
				}
			}
			return sighted.Slice(cur)[0].Count, level
		}
		return 0, lod.L2
	}
	for range 3 {
		ctx.ecs.Tick(time.Second / 60)
	}
	if n, _ := seen(); n == 0 {
		t.Fatal("near, the observer sees nothing")
	}
	eyed.look(100, 256)
	for range 3 {
		ctx.ecs.Tick(time.Second / 60)
	}
	n, level := seen()
	return n != 0, level
}
