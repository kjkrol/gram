package collision_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// a camera from above drawing what is higher further up the screen
type raising struct{ camera.Camera }

func (c raising) Project(x, y, z float32) (float32, float32) {
	sx, sy := c.Camera.Project(x, y, z)
	return sx, sy - z
}

func TestBoxes_ABaseLiesUnderItsFigureAndASolidStandsFromItsAltitudeToItsTop(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	kind.Define[struct{}](w.Kinds(), "block", kind.Spec{
		comp.Const(posAt(500, 500, 10, 10)),
		comp.Const(world.Velocity{}),
		comp.Const(world.Z{Altitude: 4, Height: 20}),
	})
	w.Write().Seed(kind.Named[struct{}](w.Kinds(), "block").Entry(struct{}{}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	boxes := c.BoxesView()
	ecs := collisiontest.Start(t, w, m, c, goke.SystemFn{OnInit: boxes.(plugin.Bound).Init})
	cam := raising{cameratest.NewFromSpace(1000, 1000, 0)}
	drawn := func(shown collision.BoxesShown) (ats []render.At, heights []float32) {
		c.Write().ShowBoxes(shown)
		boxes.(plugin.Publisher).Publish()
		boxes.(plugin.Publisher).Swap()
		var f render.Frame
		f.Reset(cam)
		boxes.Compose(&f, cam)
		f.Each(func(at render.At, v []render.Vertex) {
			ats = append(ats, at)
			for _, x := range v {
				heights = append(heights, x.DstZ)
			}
		})
		return ats, heights
	}
	drawn(collision.Bases) // the camera's view of the world is made
	ecs.Tick(time.Second / 60)

	var f render.Frame
	f.Reset(cam)
	figure := f.Lifted(505, 505, 4, world.FigureLift)
	ats, heights := drawn(collision.Bases)
	if len(ats) != 4 {
		t.Fatalf("a base of %d sides, want 4", len(ats))
	}
	for i, at := range ats {
		if at.Top() || !figure.Before(at) || at.Z() != 4+collision.BoxLift {
			t.Errorf("a base's side lies at %v, want after its figure, at %v", at,
				4+collision.BoxLift)
		}
		if heights[4*i] != 4.05 {
			t.Errorf("a base's side at the height %v, want just over its altitude", heights[4*i])
		}
	}
	ats, heights = drawn(collision.Solids)
	if len(ats) != 12 {
		t.Fatalf("a solid of %d edges, want 12", len(ats))
	}
	for _, at := range ats {
		if at != at.OverFigures() {
			t.Errorf("a solid's edge lies at %v, want over the figures", at)
		}
	}
	slices.Sort(heights)
	heights = slices.Compact(heights)
	if !slices.Equal(heights, []float32{4, 24}) {
		t.Errorf("a solid's edges at the heights %v, want its altitude and its top", heights)
	}
	if ats, _ = drawn(collision.NoBoxes); len(ats) != 0 {
		t.Errorf("%d pieces with no boxes shown", len(ats))
	}
}
