package motion_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/internal/motiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

type walker struct{ x float32 }

// two observers looking at (x, y) over a world w by h: the one a pixel a world unit, the other a
// quarter of a pixel seeing twice as far; under a ladder seeing from half a pixel a world unit,
// L0 within its sight of the point, L1 within twice that, L4 beyond
type observers [2]camera.Camera

var _ sector.Observers = (*observers)(nil)

func newObservers(w, h uint32, x, y float32) *observers {
	o := &observers{cameratest.NewFromSpace(w, h, 0), cameratest.NewFromSpace(w, h, 0)}
	for _, cam := range o {
		cam.SetViewport(2, 2)
	}
	o[1].ZoomOut(4, 0, 0)
	o[1].(interface{ See(factor float32) }).See(2)
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

func TestMove_AtL1GoesAsFarInAsLongAndNotAtAllAsleep(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4096, Height: 512},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 20},
		Sectors:  world.SectorsCfg{Buckets: 2},
		LOD:      lod.Ladder{Scales: [lod.Levels - 1]float32{0.5}, Sight: 600},
	})
	w.Write().Observe(newObservers(4096, 512, 100, 256))
	m := motion.NewPlugin(w)
	kind.Define[walker](w.Kinds(), "walker", kind.Spec{
		comp.Load(func(r walker) world.Position { return box(r.x, 250, 10) }),
		comp.Const(world.Velocity{Dir: geom.NewVec(1, 0), Value: 30}),
	})
	walkers := kind.Named[walker](w.Kinds(), "walker")
	w.Write().Seed(walkers.Entry(walker{x: 100}), walkers.Entry(walker{x: 1200}),
		walkers.Entry(walker{x: 2000}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var base goke.Comp[world.Base]
	var all *goke.Query
	ecs := motiontest.Start(t, w, m, nil, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		all = si.NewQueryBuilder(&base).Build()
	}})
	for range 8 {
		ecs.Tick(time.Second / 60)
	}
	gone := map[float32]float32{}
	for all.All(); all.Next(); {
		for _, b := range base.Slice(all.Cursor()) {
			x := b.Pos.TopLeft.X
			for _, from := range []float32{100, 1200, 2000} {
				if x >= from && x < from+50 {
					gone[from] = x - from
				}
			}
		}
	}
	want, eps := float32(8*30*(time.Second/60).Seconds()), f32.Eps(1200)
	if f32.Abs(gone[100]-want) > eps || f32.Abs(gone[1200]-want) > eps || gone[2000] != 0 {
		t.Errorf("gone at L0 %v, at L1 %v, asleep %v; want %v, %v and 0",
			gone[100], gone[1200], gone[2000], want, want)
	}
}
