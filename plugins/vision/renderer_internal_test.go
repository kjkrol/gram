package vision

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
)

// spawn materialises n entities from f.
func spawn(f *goke.Factory, n int) {
	f.Create(n)
	for f.Next() {
	}
}

func torusIf(toroidal bool) aabbworld.Edges {
	if toroidal {
		return aabbworld.Torus
	}
	return 0
}

func testSpace(t *testing.T, w, h uint32, toroidal bool) *aabbworld.Space {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{
		Width: w, Height: h, Edges: torusIf(toroidal),
		BucketSize: 256,
	})
	if err != nil {
		t.Fatal(err)
	}
	return space
}

func testRenderer(t *testing.T, w, h uint32, toroidal bool, view camera.AABB) *Renderer {
	t.Helper()
	r := NewRenderer(testSpace(t, w, h, toroidal))
	r.camera = camera.NewFromSpace(w, h, torusIf(toroidal), view)
	return r
}

func wholeWorld(w, h uint32) camera.AABB {
	return camera.AABB{TopLeft: geom.NewVec(0, 0), BottomRight: geom.NewVec(float64(w), float64(h))}
}

func TestRenderer_FanRebuildsTheAnglesFromTheIndex(t *testing.T) {
	r := testRenderer(t, 2000, 2000, false, wholeWorld(2000, 2000))
	sight := Sight{Facing: geom.NewVec(1.0, 0.0), HalfAngle: math.Pi / 4, Radius: 400}

	out := SightOutline{Count: 3}
	out.Depths[0], out.Depths[1], out.Depths[2] = 100, 200, 300

	pts := r.fan(505, 505, &sight, &out)
	if len(pts) != 4 {
		t.Fatalf("fan returned %d points, want the observer plus three samples", len(pts))
	}
	if pts[0].DstX != 505 || pts[0].DstY != 505 {
		t.Errorf("fan starts at (%v,%v), want the observer's centre (505,505)", pts[0].DstX, pts[0].DstY)
	}

	for i, want := range []struct{ angle, dist float64 }{
		{-math.Pi / 4, 100}, {0, 200}, {math.Pi / 4, 300},
	} {
		wx := 505 + want.dist*math.Cos(want.angle)
		wy := 505 + want.dist*math.Sin(want.angle)
		if math.Abs(float64(pts[i+1].DstX)-wx) > 1e-3 || math.Abs(float64(pts[i+1].DstY)-wy) > 1e-3 {
			t.Errorf("sample %d at (%v,%v), want (%.3f,%.3f)", i, pts[i+1].DstX, pts[i+1].DstY, wx, wy)
		}
	}
}

func TestRenderer_QueryVisitsOnlyEntitiesWithAnOutline(t *testing.T) {
	r := testRenderer(t, 2000, 2000, false, wholeWorld(2000, 2000))

	var withOutline, withoutOutline goke.Comp[world.Base]
	var sight goke.Comp[Sight]
	var outline goke.Comp[SightOutline]

	ecs := goke.New()
	ecs.Setup(
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			spawn(si.NewFactory(&withOutline, &sight, &outline), 1)
		}},
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			spawn(si.NewFactory(&withoutOutline, &sight), 2)
		}},
		goke.SystemFn{OnInit: r.Init},
	)

	seen := 0
	r.query.All()
	for r.query.Next() {
		seen += len(r.query.Cursor().IDs)
	}
	if seen != 1 {
		t.Errorf("renderer query matched %d entities, want only the one carrying SightOutline", seen)
	}
}

func TestRenderer_DrawSkipsShortOutlinesAndOffscreenEntities(t *testing.T) {
	r := testRenderer(t, 4000, 4000, false, camera.AABB{
		TopLeft:     geom.NewVec(0, 0),
		BottomRight: geom.NewVec(1000, 1000),
	})

	drawn := 0
	r.WithStyle(ConeStyleFn(func(*ebiten.Image, []ebiten.Vertex) { drawn++ }))

	var pos goke.Comp[world.Base]
	var sight goke.Comp[Sight]
	var outline goke.Comp[SightOutline]

	good := SightOutline{Count: 3}
	good.Depths[0], good.Depths[1], good.Depths[2] = 50, 60, 70

	place := func(si *goke.SysInit, at geom.Vec, o SightOutline) {
		f := si.NewFactory(&pos, &sight, &outline)
		f.Create(1)
		for f.Next() {
			pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: plane.NewAABB(at, 10, 10)}
			sight.Slice(&f.Cursor)[0] = Sight{Facing: geom.NewVec(1.0, 0.0), HalfAngle: 0.5, Radius: 100}
			outline.Slice(&f.Cursor)[0] = o
		}
	}

	ecs := goke.New()
	ecs.Setup(
		goke.SystemFn{OnInit: func(si *goke.SysInit) { place(si, geom.NewVec(100, 100), good) }},
		goke.SystemFn{OnInit: func(si *goke.SysInit) { place(si, geom.NewVec(200, 200), SightOutline{Count: 1}) }},
		goke.SystemFn{OnInit: func(si *goke.SysInit) { place(si, geom.NewVec(3000, 3000), good) }},
		goke.SystemFn{OnInit: r.Init},
	)

	r.DrawWorld(nil, r.camera)
	if drawn != 1 {
		t.Errorf("drew %d cones, want 1 — the short outline and the offscreen entity should both be skipped", drawn)
	}
}

// recordFans collects a copy of every fan the renderer hands to the style.
func recordFans(r *Renderer) *[][]ebiten.Vertex {
	var fans [][]ebiten.Vertex
	r.WithStyle(ConeStyleFn(func(_ *ebiten.Image, pts []ebiten.Vertex) {
		fans = append(fans, append([]ebiten.Vertex(nil), pts...))
	}))
	return &fans
}

// drawAt draws one entity with a full cone at (x, y) and returns every fan that reached the style.
func drawAt(t *testing.T, r *Renderer, x, y float64, radius float64) [][]ebiten.Vertex {
	t.Helper()
	fans := recordFans(r)

	var pos goke.Comp[world.Base]
	var sight goke.Comp[Sight]
	var outline goke.Comp[SightOutline]

	o := SightOutline{Count: 9}
	for i := range int(o.Count) {
		o.Depths[i] = float32(radius)
	}

	ecs := goke.New()
	ecs.Setup(
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			f := si.NewFactory(&pos, &sight, &outline)
			f.Create(1)
			for f.Next() {
				pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: plane.NewAABB(geom.NewVec(x, y), 10, 10)}
				sight.Slice(&f.Cursor)[0] = Sight{
					Facing: geom.NewVec(1.0, 0.0), HalfAngle: math.Pi, Radius: radius,
				}
				outline.Slice(&f.Cursor)[0] = o
			}
		}},
		goke.SystemFn{OnInit: r.Init},
	)
	r.DrawWorld(nil, r.camera)
	return *fans
}

func TestRenderer_NoFanEdgeSpansTheScreen(t *testing.T) {
	const radius = 200.0

	for name, x := range map[string]float64{"on the seam": 995, "well inside": 500} {
		t.Run(name, func(t *testing.T) {
			r := testRenderer(t, 1000, 1000, true, wholeWorld(1000, 1000))
			limit := float32(2*radius) * r.camera.Zoom()

			for f, fan := range drawAt(t, r, x, 500, radius) {
				for i := 1; i < len(fan); i++ {
					dx := fan[i].DstX - fan[i-1].DstX
					dy := fan[i].DstY - fan[i-1].DstY
					if d := math.Hypot(float64(dx), float64(dy)); d > float64(limit)+1e-3 {
						t.Fatalf("fan %d edge %d is %.1f long, over the %.1f a cone can span — the shape was torn at the seam", f, i, d, limit)
					}
				}
			}
		})
	}
}

func TestRenderer_DrawsOneCopyPerWorldImage(t *testing.T) {
	const radius = 200.0

	cases := map[string]struct {
		toroidal bool
		x, y     float64
		want     int
	}{
		"middle of a toroidal world": {true, 500, 500, 1},
		"across the vertical seam":   {true, 990, 500, 2},
		"across the horizontal seam": {true, 500, 990, 2},
		"in the corner":              {true, 990, 990, 4},
		"euclidean world":            {false, 990, 990, 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := testRenderer(t, 1000, 1000, tc.toroidal, wholeWorld(1000, 1000))
			if got := len(drawAt(t, r, float64(tc.x), float64(tc.y), radius)); got != tc.want {
				t.Errorf("drew %d copies, want %d", got, tc.want)
			}
		})
	}
}

// A wrapped copy is the same shape as the original, moved by exactly one world.
func TestRenderer_WrappedCopyIsTheOriginalShifted(t *testing.T) {
	r := testRenderer(t, 1000, 1000, true, wholeWorld(1000, 1000))
	fans := drawAt(t, r, 990, 500, 200)
	if len(fans) != 2 {
		t.Fatalf("drew %d copies, want 2", len(fans))
	}

	shift := float32(1000) * r.camera.Zoom()
	for i := range fans[0] {
		if dx := fans[0][i].DstX - fans[1][i].DstX; math.Abs(float64(dx-shift)) > 1e-3 {
			t.Fatalf("point %d differs by %.3f across, want exactly one world (%.3f)", i, dx, shift)
		}
		if dy := fans[0][i].DstY - fans[1][i].DstY; math.Abs(float64(dy)) > 1e-3 {
			t.Fatalf("point %d differs by %.3f down, want none", i, dy)
		}
	}
}
