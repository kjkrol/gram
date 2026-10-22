package motion_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/internal/motiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

type walker struct{ x float64 }

type anchor struct{ x, y float32 }

func (a anchor) Anchors(at func(x, y float32)) { at(a.x, a.y) }

var _ sector.Anchors = anchor{}

func TestMove_InAFartherBandGoesAsFarInAsLongAndNotAtAllBeyond(t *testing.T) {
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
	w.Write().Anchor(anchor{x: 100, y: 256})
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
	gone := map[float64]float64{}
	for all.All(); all.Next(); {
		for _, b := range base.Slice(all.Cursor()) {
			x := b.Pos.TopLeft.X
			for _, from := range []float64{100, 1200, 2000} {
				if x >= from && x < from+50 {
					gone[from] = x - from
				}
			}
		}
	}
	want := 8 * 30 * (time.Second / 60).Seconds()
	if math.Abs(gone[100]-want) > 1e-6 || math.Abs(gone[1200]-want) > 1e-6 || gone[2000] != 0 {
		t.Errorf("gone near %v, in the second band %v, beyond %v; want %v, %v and 0",
			gone[100], gone[1200], gone[2000], want, want)
	}
}
