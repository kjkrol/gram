package vision

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/tag"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
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
	r.camera = icamera.NewFromSpace(w, h, torusIf(toroidal), view)
	return r
}

func wholeWorld(w, h uint32) camera.AABB {
	return camera.AABB{TopLeft: geom.NewVec(0, 0), BottomRight: geom.NewVec(float64(w), float64(h))}
}

func TestRenderer_FanRebuildsTheAnglesFromTheIndex(t *testing.T) {
	r := testRenderer(t, 2000, 2000, false, wholeWorld(2000, 2000))
	sight := Sight{Facing: geom.NewVec(1.0, 0.0), Radius: 400}

	out := SightOutline{Count: 3}
	out.Depths[0], out.Depths[1], out.Depths[2] = 100, 200, 300

	pts := r.fan(505, 505, math.Pi/4, &sight, &out)
	if len(pts) != 4 {
		t.Fatalf("fan returned %d points, want the observer plus three samples", len(pts))
	}
	if pts[0].X != 505 || pts[0].Y != 505 {
		t.Errorf("fan starts at (%v,%v), want the observer's centre (505,505)", pts[0].X, pts[0].Y)
	}

	for i, want := range []struct{ angle, dist float64 }{
		{-math.Pi / 4, 100}, {0, 200}, {math.Pi / 4, 300},
	} {
		wx := 505 + want.dist*math.Cos(want.angle)
		wy := 505 + want.dist*math.Sin(want.angle)
		if math.Abs(float64(pts[i+1].X)-wx) > 1e-3 || math.Abs(float64(pts[i+1].Y)-wy) > 1e-3 {
			t.Errorf("sample %d at (%v,%v), want (%.3f,%.3f)", i, pts[i+1].X, pts[i+1].Y, wx, wy)
		}
	}
}

// viewers is a tag family for the tests.
type viewers struct{}

// Composed, only the views of the observers carrying an outline are drawn; with a Show rule
// for the ones tagged, only theirs.
func TestRenderer_ComposesTheOutlinedViewsTheShowRulesShow(t *testing.T) {
	every := testRenderer(t, 2000, 2000, false, wholeWorld(2000, 2000))
	tagged := testRenderer(t, 2000, 2000, false, wholeWorld(2000, 2000))
	shown := tag.Tag[viewers](3)
	var rules render.Rules
	if err := rules.Add(render.Show(shown.In)); err != nil {
		t.Fatal(err)
	}
	tagged.WithDrawing(&rules)
	drawn := map[*Renderer]int{}
	for _, r := range []*Renderer{every, tagged} {
		r.WithStyle(ConeStyleFn(func(*render.Frame, []ConePoint) { drawn[r]++ }))
	}

	var base goke.Comp[world.Base]
	var sight goke.Comp[Sight]
	var eye goke.Comp[world.Eye]
	var outline goke.Comp[SightOutline]
	var tags goke.Comp[tag.Tags[viewers]]
	good := SightOutline{Count: 3}
	good.Depths[0], good.Depths[1], good.Depths[2] = 50, 60, 70
	place := func(f *goke.Factory, at geom.Vec, marks tag.Tags[viewers]) {
		f.Create(1)
		for f.Next() {
			base.Slice(&f.Cursor)[0].Pos = world.Position{AABB: plane.NewAABB(at, 10, 10)}
			sight.Slice(&f.Cursor)[0] = Sight{Facing: geom.NewVec(1.0, 0.0), Radius: 100}
			eye.Slice(&f.Cursor)[0] = world.Eye{Angle: 1}
			outline.Slice(&f.Cursor)[0] = good
			tags.Slice(&f.Cursor)[0] = marks
		}
	}
	ecs := goke.New()
	ecs.Setup(
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			f := si.NewFactory(&base, &sight, &eye, &outline, &tags)
			place(f, geom.NewVec(100, 100), tag.Tags[viewers](0).With(shown))
			place(f, geom.NewVec(300, 300), 0)
		}},
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			spawn(si.NewFactory(&base, &sight, &eye), 2)
		}},
		goke.SystemFn{OnInit: every.Init},
		goke.SystemFn{OnInit: tagged.Init},
	)
	composeWith(every)
	composeWith(tagged)
	if drawn[every] != 2 {
		t.Errorf("without a Show rule %d views were drawn, want the two with an outline", drawn[every])
	}
	if drawn[tagged] != 1 {
		t.Errorf("showing the tagged, %d views were drawn, want the one tagged", drawn[tagged])
	}
}

func TestRenderer_DrawSkipsShortOutlinesAndOffscreenEntities(t *testing.T) {
	r := testRenderer(t, 4000, 4000, false, camera.AABB{
		TopLeft:     geom.NewVec(0, 0),
		BottomRight: geom.NewVec(1000, 1000),
	})

	drawn := 0
	r.WithStyle(ConeStyleFn(func(*render.Frame, []ConePoint) { drawn++ }))

	var pos goke.Comp[world.Base]
	var sight goke.Comp[Sight]
	var eye goke.Comp[world.Eye]
	var outline goke.Comp[SightOutline]

	good := SightOutline{Count: 3}
	good.Depths[0], good.Depths[1], good.Depths[2] = 50, 60, 70

	place := func(si *goke.SysInit, at geom.Vec, o SightOutline) {
		f := si.NewFactory(&pos, &sight, &eye, &outline)
		f.Create(1)
		for f.Next() {
			pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: plane.NewAABB(at, 10, 10)}
			sight.Slice(&f.Cursor)[0] = Sight{Facing: geom.NewVec(1.0, 0.0), Radius: 100}
			eye.Slice(&f.Cursor)[0] = world.Eye{Angle: 1}
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

	composeWith(r)
	if drawn != 1 {
		t.Errorf("drew %d cones, want 1 — the short outline and the offscreen entity should both be skipped", drawn)
	}
}

func TestRenderer_HiddenComposesNothing(t *testing.T) {
	r := testRenderer(t, 4000, 4000, false, camera.AABB{
		TopLeft:     geom.NewVec(0, 0),
		BottomRight: geom.NewVec(1000, 1000),
	})
	drawn := 0
	r.WithStyle(ConeStyleFn(func(*render.Frame, []ConePoint) { drawn++ }))

	var pos goke.Comp[world.Base]
	var sight goke.Comp[Sight]
	var eye goke.Comp[world.Eye]
	var outline goke.Comp[SightOutline]
	good := SightOutline{Count: 3}
	good.Depths[0], good.Depths[1], good.Depths[2] = 50, 60, 70

	ecs := goke.New()
	ecs.Setup(
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			f := si.NewFactory(&pos, &sight, &eye, &outline)
			f.Create(1)
			for f.Next() {
				pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}
				sight.Slice(&f.Cursor)[0] = Sight{Facing: geom.NewVec(1.0, 0.0), Radius: 100}
				eye.Slice(&f.Cursor)[0] = world.Eye{Angle: 1}
				outline.Slice(&f.Cursor)[0] = good
			}
		}},
		goke.SystemFn{OnInit: r.Init},
	)

	r.Hide(true)
	composeWith(r)
	if drawn != 0 {
		t.Errorf("hidden, the renderer drew %d cones, want none", drawn)
	}
	r.Hide(false)
	composeWith(r)
	if drawn != 1 {
		t.Errorf("shown again, the renderer drew %d cones, want 1", drawn)
	}
}

// recordFans collects a copy of every ring the renderer hands to the style.
func recordFans(r *Renderer) *[][]ConePoint {
	var fans [][]ConePoint
	r.WithStyle(ConeStyleFn(func(_ *render.Frame, pts []ConePoint) {
		fans = append(fans, append([]ConePoint(nil), pts...))
	}))
	return &fans
}

// composeWith composes r through its camera into a fresh frame.
func composeWith(r *Renderer) *render.Frame {
	var f render.Frame
	f.Reset(r.camera)
	r.Compose(&f, r.camera)
	return &f
}

// drawAt draws one entity with a full cone at (x, y) and returns every fan that reached the style.
func drawAt(t *testing.T, r *Renderer, x, y float64, radius float64) [][]ConePoint {
	t.Helper()
	fans := recordFans(r)

	var pos goke.Comp[world.Base]
	var sight goke.Comp[Sight]
	var eye goke.Comp[world.Eye]
	var outline goke.Comp[SightOutline]

	o := SightOutline{Count: 9}
	for i := range int(o.Count) {
		o.Depths[i] = float32(radius)
	}

	ecs := goke.New()
	ecs.Setup(
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			f := si.NewFactory(&pos, &sight, &eye, &outline)
			f.Create(1)
			for f.Next() {
				pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: plane.NewAABB(geom.NewVec(x, y), 10, 10)}
				sight.Slice(&f.Cursor)[0] = Sight{Facing: geom.NewVec(1.0, 0.0), Radius: radius}
				eye.Slice(&f.Cursor)[0] = world.Eye{Angle: 2 * math.Pi}
				outline.Slice(&f.Cursor)[0] = o
			}
		}},
		goke.SystemFn{OnInit: r.Init},
	)
	composeWith(r)
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
					dx := fan[i].X - fan[i-1].X
					dy := fan[i].Y - fan[i-1].Y
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
		if dx := fans[0][i].X - fans[1][i].X; math.Abs(float64(dx-shift)) > 1e-3 {
			t.Fatalf("point %d differs by %.3f across, want exactly one world (%.3f)", i, dx, shift)
		}
		if dy := fans[0][i].Y - fans[1][i].Y; math.Abs(float64(dy)) > 1e-3 {
			t.Fatalf("point %d differs by %.3f down, want none", i, dy)
		}
	}
}

// slope is a Ground rising to the east, sampled every 10.
type slope struct{}

func (slope) At(p geom.Vec) float64 { return p.X / 10 }
func (slope) Step() float64         { return 10 }

// isoRenderer is a renderer over a 1000x1000 world of slope, through an isometric camera.
func isoRenderer(t *testing.T) *Renderer {
	t.Helper()
	r := NewRenderer(testSpace(t, 1000, 1000, false)).WithGround(func() ground.Heights { return slope{} })
	r.camera = isoCamera(1000, 1000, camera.Config{ViewportWidth: 800, ViewportHeight: 600})
	r.camera.MoveTo(0, 0)
	r.ground, r.step, r.grounded = slope{}, 10, true
	return r
}

func TestRenderer_DrapesTheRingOverTheGroundInItsSteps(t *testing.T) {
	r := isoRenderer(t)
	sight := Sight{Facing: geom.NewVec(1, 0), Radius: 50}
	o := SightOutline{Count: 3}
	o.Depths[0], o.Depths[1], o.Depths[2] = 45, 50, 25
	ring := r.draped(100, 100, 7, 0.2, &sight, &o)

	// the apex, the near edge every 10 short of 45, the three reaches, the far edge every 10 short of 25
	if len(ring) != 1+4+3+2 {
		t.Fatalf("a ring of %d points, want 10", len(ring))
	}
	if ring[0].Depth != r.camera.Depth(100, 100, 7) {
		t.Errorf("the apex at depth %v, want the observer's at its altitude", ring[0].Depth)
	}
	a := -0.2
	x, y := 100+20*float32(math.Cos(a)), 100+20*float32(math.Sin(a))
	if p := ring[2]; p.Depth != r.camera.Depth(x, y, x/10) {
		t.Errorf("the edge point 20 out at depth %v, want the ground's there %v", p.Depth, r.camera.Depth(x, y, x/10))
	}
	from := func(p ConePoint) float64 { return math.Hypot(float64(p.X-ring[0].X), float64(p.Y-ring[0].Y)) }
	if from(ring[8]) <= from(ring[9]) {
		t.Errorf("the far edge runs outwards (%v then %v from the apex), want it back towards the observer", from(ring[8]), from(ring[9]))
	}
}

func TestRenderer_ShadowsFadeOnlyWhereTheyMeetGroundInSight(t *testing.T) {
	r := isoRenderer(t)
	f := new(render.Frame)
	f.Reset(r.camera)
	r.frame = f
	sight := Sight{Facing: geom.NewVec(1, 0), Radius: 100}
	o := SightOutline{Count: 5}
	for i := 1; i <= 3; i++ {
		o.Shadows[i][0] = Band{From: 40, To: 60} // two steps of ground at each of three angles
	}
	r.shade(100, 100, 0.2, &sight, &o)

	var pieces [][]float32 // per piece: left, right, top, bottom fade distance at its first corner
	f.Each(func(tier render.Tier, _ float32, v []render.Vertex) {
		if tier != render.Overlays {
			t.Errorf("a shadow on tier %d, want Overlays", tier)
		}
		pieces = append(pieces, []float32{v[0].Custom0, v[3].Custom1, v[0].Custom2, v[3].Custom3})
	})
	if len(pieces) != 6 {
		t.Fatalf("%d shadow pieces, want three angles of two steps", len(pieces))
	}
	const hard = 0 // an edge that does not fade
	for k, p := range pieces {
		angle, near := k/2, k%2 == 0
		if left := p[0] != hard; left != (angle == 0) {
			t.Errorf("piece %d fades on its left %v, want only the first angle's", k, left)
		}
		if right := p[1] != hard; right != (angle == 2) {
			t.Errorf("piece %d fades on its right %v, want only the last angle's", k, right)
		}
		if top := p[2] != hard; top != near {
			t.Errorf("piece %d fades at its near end %v, want only the nearer piece", k, top)
		}
		if bottom := p[3] != hard; bottom == near {
			t.Errorf("piece %d fades at its far end %v, want only the farther piece", k, bottom)
		}
	}
}

// isoCamera is a camera of a width x height world put in the isometric view.
func isoCamera(width, height uint32, cfg camera.Config) camera.Camera {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 100},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(width/32, height/32, 32), &cell.MultipleOccupancy{}, w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true})
	return cameras.NewPlugin(w, p.Views(), cfg).Main()
}

// behindUnder is a camera with nothing in front of its eye short of x 146: as a perspective riding
// low has the ground under and behind it.
type behindUnder struct{ camera.Camera }

func (behindUnder) ScaleAt(x, _, _ float32) float32 {
	if x < 146 {
		return 0
	}
	return 2
}

// A piece of shadow not wholly in front of the eye is left out — it lies under or behind it,
// where it would be thrown across the screen — and the rest is drawn.
func TestRenderer_ShadowsLeaveOutWhatIsNotInFrontOfTheEye(t *testing.T) {
	r := isoRenderer(t)
	r.camera = behindUnder{r.camera}
	f := new(render.Frame)
	f.Reset(r.camera)
	r.frame = f
	sight := Sight{Facing: geom.NewVec(1, 0), Radius: 100}
	o := SightOutline{Count: 5}
	for i := 1; i <= 3; i++ {
		o.Shadows[i][0] = Band{From: 40, To: 60} // two steps of ground at each of three angles
	}
	r.shade(100, 100, 0.2, &sight, &o)
	n := 0
	f.Each(func(render.Tier, float32, []render.Vertex) { n++ })
	if n != 3 {
		t.Errorf("%d shadow pieces, want the three farther ones: the nearer reach back short of x 146", n)
	}
}
