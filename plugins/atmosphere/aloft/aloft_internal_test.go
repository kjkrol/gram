package aloft

import (
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/backdrop"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// half heaped clouds over the west half of a world 640 by 480, the east clear, a heap drifted
// over the west's middle: they lie a heap's cell apart, wider than the world
func westCloudy() air.Weather {
	d := air.NewDome(640, 480, 2000)
	d.Own()
	d.Carry([2]float32{}, func(x, _ float32) (air.Mass, int16) {
		if x < 320 {
			return air.Mass{Cover: 0.9, Billow: 0.5}, 0
		}
		return air.Mass{}, 0
	}, true)
	var still air.Weather
	most, heap := float32(0), [2]float32{}
	for i := range 64 {
		for j := range 64 {
			x, y := float32(i)*50, float32(j)*50
			if n := still.Cloud(air.Low, x, y); n > most {
				most, heap = n, [2]float32{x, y}
			}
		}
	}
	return air.Weather{Dome: d, Temperature: 10, Bases: air.Bases(world.Scale{}),
		Drift: [2]float32{160 - heap[0], 240 - heap[1]}}
}

// a view from above whose lines of sight start under the clouds, over the highest ground
type low struct{ camera.Camera }

func (l low) Rays() (camera.RayField, bool) {
	f, ok := l.Camera.(camera.Rays).Rays()
	f.Origin[2] = 50
	return f, ok
}

type sunk struct {
	camera.Camera
	z float32
}

// a perspective eye at height z, looking down
func (s sunk) Rays() (camera.RayField, bool) {
	return camera.RayField{Origin: [3]float32{320, 240, s.z}, Dir: [3]float32{0, 0, -1},
		DDX: [3]float32{0.01, 0, 0}, DDY: [3]float32{0, 0.01, 0}}, true
}

// the uniforms the frame hands a renderer of the weather now, the dome's picture and the tile too
func framed(t *testing.T, w *world.Plugin, cam camera.Camera, weather func() air.Weather,
	r *Renderer, more ...render.Source) render.Uniforms {
	t.Helper()
	return render.UniformsOf(framedBy(w, cam, weather, r, more...))
}

func framedBy(w *world.Plugin, cam camera.Camera, weather func() air.Weather, r *Renderer,
	more ...render.Source) map[string]any {
	sun := func() light.Distant { return sky.DefaultSun }
	sources := append([]render.Source{backdrop.New(w.Res.Config.Space, sun, weather), r}, more...)
	c := render.NewComposer(sources...)
	c.DrawWorld(nil, cam)
	u := map[string]any{}
	for name, v := range c.Uniforms() {
		u[name] = v
	}
	for _, s := range more {
		if g, ok := s.(heights); ok {
			u["GroundHeights"] = g.Image
		}
	}
	var masses air.DomeImage
	if d := weather().Dome; d != nil {
		u["AirMasses"] = masses.Of(d)
	}
	u["CloudTile"] = air.Tile()
	return u
}

// how much of the columns x0 to x1 of a view 640 by 480 the clouds r draws through cam cover
func covering(r *Renderer, t render.Target, cam camera.Camera, u render.Uniforms,
	x0, x1 int) float64 {
	screen := render.NewImage(640, 480)
	defer screen.Deallocate()
	t.Screen = screen
	r.Draw(t, cam, u)
	pix := make([]byte, 4*640*480)
	screen.ReadPixels(pix)
	sum := 0.0
	for y := range 480 {
		for x := x0; x < x1; x++ {
			sum += float64(pix[4*(y*640+x)+3])
		}
	}
	return sum / float64(255*480*(x1-x0))
}

func TestRenderer_DrawsTheCloudsFromAboveOverTheirMassAlone(t *testing.T) {
	gputest.Need(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w, cameras.Config{}).Write().New(cameras.FromAbove, camera.Config{})
	cam.SetViewport(640, 480)
	weather := westCloudy()
	weatherNow := func() air.Weather { return weather }
	bases := weather.Bases
	for _, layer := range []air.Layer{air.Low, air.High} {
		r := New(weatherNow, layer)
		if _, _, ok := r.field(cam); ok {
			t.Fatal("hidden, the clouds are drawn from above")
		}
		r.Show(true)
		u := framed(t, w, cam, weatherNow, r)
		// the noise leaves gaps: the west half covered on the whole, the east bare
		for _, through := range []camera.Camera{cam, low{cam}} {
			west := covering(r, render.Target{}, through, u, 0, 280)
			east := covering(r, render.Target{}, through, u, 360, 640)
			if west < 0.1 || east > 0.01 {
				t.Errorf("layer %d seen from above through %T: the west is covered %.2f and the "+
					"east %.2f, want the clouds over the west alone", layer, through, west, east)
			}
		}
		for _, c := range []struct {
			z    float32
			seen bool
		}{{100, false}, {(bases[air.Low] + bases[air.High]) / 2, layer == air.Low}, {9000, true}} {
			if _, _, ok := r.field(sunk{Camera: cam, z: c.z}); ok != c.seen {
				t.Errorf("layer %d from an eye %v up: seen from above %v, want %v", layer, c.z, ok,
					c.seen)
			}
		}
	}
}

func TestRenderer_InOneLayerDrawsTheLowOnesHighAlone(t *testing.T) {
	w := westCloudy()
	w.OneLayer = true
	weatherNow := func() air.Weather { return w }
	cam := cameras.NewPlugin(world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 640, Height: 480},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8},
	}), cameras.Config{}).Write().New(cameras.FromAbove, camera.Config{})
	low, high := New(weatherNow, air.Low), New(weatherNow, air.High)
	low.Show(true)
	high.Show(true)
	if _, _, ok := high.field(cam); ok {
		t.Error("in one layer the high clouds are drawn from above")
	}
	between := (w.Bases[air.Low] + w.Bases[air.High]) / 2
	if _, _, ok := low.field(sunk{Camera: cam, z: between}); ok {
		t.Errorf("in one layer an eye %v up sees the clouds from above, want them %v up",
			between, w.Bases[air.High])
	}
	if _, _, ok := low.field(cam); !ok || low.base() != w.Bases[air.High] {
		t.Errorf("in one layer the clouds from above are drawn %v, %v up; want %v up", ok,
			low.base(), w.Bases[air.High])
	}
}

// the ground's heights handed to the frame
type heights struct{ render.GroundHeights }

func (heights) Init(*goke.SysInit)                         {}
func (h heights) Compose(f *render.Frame, _ camera.Camera) { f.GroundHeights(h.GroundHeights) }

// a world 640 by 480 whose west half stands 2000 up, over the low clouds' heaps, the east at 0
func westRaised() heights {
	const cols, rows = 65, 49
	img := render.NewImage(cols, rows)
	pix := make([]byte, 0, 4*cols*rows)
	for range rows {
		for x := range cols {
			v := byte(0)
			if x < 32 {
				v = 255
			}
			pix = append(pix, v, v, 0, 255)
		}
	}
	img.WritePixels(pix)
	return heights{render.GroundHeights{Image: img, Corners: [2]int{cols, rows}, Cell: 10,
		Span: 2000}}
}

func TestRenderer_HidesTheCloudsInsideTheGround(t *testing.T) {
	gputest.Need(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w, cameras.Config{}).Write().New(cameras.FromAbove, camera.Config{})
	cam.SetViewport(640, 480)
	ground := westRaised()
	for _, layer := range []air.Layer{air.Low, air.High} {
		// all of the cover in the layer
		weather := air.Weather{Clouds: 0.9, Billow: float32(1 - layer),
			Bases: air.Bases(world.Scale{})}
		weatherNow := func() air.Weather { return weather }
		r := New(weatherNow, layer)
		r.Show(true)
		r.Bodies(true)
		u := framed(t, w, cam, weatherNow, r, ground)
		west := covering(r, render.Target{}, cam, u, 0, 300)
		east := covering(r, render.Target{}, cam, u, 340, 640)
		hidden := layer == air.Low
		if east < 0.1 || (west < 0.01) != hidden || (west > 0.1) == hidden {
			t.Errorf("layer %d over ground 2000 up in the west, 0 in the east: covered %.2f in "+
				"the west, %.2f in the east; want hidden in the west %v", layer, west, east, hidden)
		}
	}
}

// a perspective eye at (600, 240, z) looking west, up at the top of a view 640 by 480 and down
// at its bottom
type level struct {
	camera.Camera
	z float32
}

func (l level) Rays() (camera.RayField, bool) {
	return camera.RayField{Origin: [3]float32{600, 240, l.z}, Dir: [3]float32{-1, -0.32, 0.24},
		DDX: [3]float32{0, 0.001, 0}, DDY: [3]float32{0, 0, -0.001}}, true
}

// how much of the rows y0 to y1 of a view 640 by 480 the clouds r draws through cam cover
func coveringRows(r *Renderer, t render.Target, cam camera.Camera, u render.Uniforms,
	y0, y1 int) float64 {
	screen := render.NewImage(640, 480)
	defer screen.Deallocate()
	if t.Depth != nil {
		screen.ClearDepth(t.Depth)
	}
	t.Screen = screen
	r.Draw(t, cam, u)
	pix := make([]byte, 4*640*480)
	screen.ReadPixels(pix)
	sum := 0.0
	for y := y0; y < y1; y++ {
		for x := range 640 {
			sum += float64(pix[4*(y*640+x)+3])
		}
	}
	return sum / float64(255*640*(y1-y0))
}

// what stands nearest everywhere, in the depth buffer
var nearest = render.NewMeshShaderWith("nearest", []byte(`
struct Out { @builtin(position) clip: vec4<f32> }
struct Laid { @location(0) color: vec4<f32>, @builtin(frag_depth) depth: f32 }
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(3.0, -1.0),
        vec2<f32>(-1.0, 3.0),
    );
    return Out(vec4<f32>(corners[vid], 0.0, 1.0));
}
@fragment
fn fs_main(o: Out) -> Laid { return Laid(vec4<f32>(0.0), 1.0); }
`), nil)

func TestRenderer_DrawsTheLowCloudsBeforeTheSlopesFromUnderThem(t *testing.T) {
	gputest.Need(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w, cameras.Config{}).Write().New(cameras.FromAbove, camera.Config{})
	cam.SetViewport(640, 480)
	relief := air.ReliefOf(world.Scale{}, 0, 1860)
	weather := air.Weather{Clouds: 0.9, Billow: 1, Relief: relief,
		Bases: air.BasesOver(world.Scale{}, relief), Ceilings: [3]float32{1, 0, 0}}
	weatherNow := func() air.Weather { return weather }
	r := New(weatherNow, air.Low)
	r.Show(true)
	r.Bodies(true)
	eye := level{Camera: cam, z: 1250}
	if _, under, ok := r.field(eye); !ok || !under {
		t.Fatalf("from an eye 1250 up, under the clouds' top %v: drawn %v, from under them %v; "+
			"want both", weather.Top(air.Low), ok, under)
	}
	wall := framed(t, w, eye, weatherNow, r, westRaised())
	up := coveringRows(r, render.Target{}, eye, wall, 0, 120)
	down := coveringRows(r, render.Target{}, eye, wall, 360, 480)
	if up < 0.1 || down > 0.01 {
		t.Errorf("looking at a wall 2000 high through clouds 1265 up: covered %.2f above, %.2f "+
			"below; want the clouds before the wall above alone", up, down)
	}
	bare := New(weatherNow, air.Low)
	bare.Show(true)
	bare.Bodies(true)
	flat := framed(t, w, eye, weatherNow, bare)
	if got := coveringRows(bare, render.Target{}, eye, flat, 0, 120); got > 0.01 {
		t.Errorf("with no ground standing up into the clouds covered %.2f above, want none: the "+
			"sky has them", got)
	}
	depth := render.NewDepth()
	screen := render.NewImage(640, 480)
	defer screen.Deallocate()
	screen.DrawMesh(nil, nearest, &render.DrawMeshOptions{Vertices: 3, Depth: depth,
		ClearDepth: true, WriteDepth: true})
	r.Draw(render.Target{Screen: screen, Depth: depth}, eye, wall)
	pix := make([]byte, 4*640*480)
	screen.ReadPixels(pix)
	hidden := 0.0
	for i := 3; i < len(pix); i += 4 {
		hidden += float64(pix[i])
	}
	if hidden > 0 {
		t.Errorf("behind what stands nearest everywhere the clouds still show, %v of alpha", hidden)
	}
}

func TestRenderer_DrawsTheCloudsAtACoarseOfTheGroundsPixels(t *testing.T) {
	gputest.Need(t)
	r := New(func() air.Weather { return air.Weather{} }, air.Low)
	for _, c := range []struct {
		coarse, sw, sh int
		ground         float32
		w, h           int
	}{
		{4, 640, 480, 0, 160, 120},
		{8, 640, 480, 0, 80, 60},
		{6, 640, 480, 0, 107, 80},
		{4, 640, 480, 2, 80, 60},
		{4, 2560, 1440, 2560.0 / 1536, 384, 216},
	} {
		r.Coarse(c.coarse)
		b := r.sizeFor(c.sw, c.sh, c.ground).coarse.Bounds()
		if b.Dx() != c.w || b.Dy() != c.h {
			t.Errorf("at a coarse of %v the clouds of a screen %v by %v over a ground of %v of "+
				"its pixels a pixel are %v by %v, want %v by %v", c.coarse, c.sw, c.sh, c.ground,
				b.Dx(), b.Dy(), c.w, c.h)
		}
	}
	if len(r.sizes) != 4 {
		t.Errorf("%v sizes kept, want 4: one a size of the clouds on a screen, at a coarse of 8 "+
			"and over a ground at half the pixels at 4 the same", len(r.sizes))
	}
}

// a ground handing the frame the uniforms and images u, opaque and drawn nowhere, so what lies
// over it meets the furthest depth
type handed map[string]any

func (handed) Init(*goke.SysInit)                                 {}
func (handed) Draw(render.Target, camera.Camera, render.Uniforms) {}
func (h handed) Compose(f *render.Frame, _ camera.Camera) {
	f.DrawOpaque()
	for name, v := range h {
		switch v := v.(type) {
		case []float32:
			if name != "Toward" && name != "Clock" && name != "Pixel" {
				f.Uniform(name, v...)
			}
		case *render.Image:
			f.Image(name, v)
		}
	}
}

func TestRenderer_OverAGroundAtHalfThePixelsDrawsTheCloudsAtItsPixelsOnceAFrame(t *testing.T) {
	gputest.Need(t)
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 640, Height: 480},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}})
	cam := cameras.NewPlugin(w, cameras.Config{}).Write().New(cameras.FromAbove, camera.Config{})
	cam.SetViewport(640, 480)
	weather := westCloudy()
	weatherNow := func() air.Weather { return weather }
	r := New(weatherNow, air.Low)
	r.Show(true)
	r.Bodies(true)
	u := framedBy(w, cam, weatherNow, r)
	cam.(camera.Resolved).SetResolution(0.5)
	cam.(camera.Figured).SetSharpFigures(true)
	feed := render.NewFeed(cam, render.NewComposer(handed(u), r))
	screen := render.NewImage(640, 480)
	defer screen.Deallocate()
	for range 2 {
		screen.Clear()
		feed.DrawOn(screen)
	}
	if len(r.sizes) != 1 || r.sizes[0].cw != 80 || r.sizes[0].ch != 60 {
		var drawn [][2]int
		for _, s := range r.sizes {
			drawn = append(drawn, [2]int{s.cw, s.ch})
		}
		t.Errorf("over a ground at half the pixels of a screen 640 by 480 the clouds were drawn "+
			"at %v, want once at 80 by 60: a quarter of the ground's pixels each way", drawn)
	}
	pix := make([]byte, 4*640*480)
	screen.ReadPixels(pix)
	cover := func(x0, x1 int) float64 {
		sum := 0.0
		for y := range 480 {
			for x := x0; x < x1; x++ {
				sum += float64(pix[4*(y*640+x)+3])
			}
		}
		return sum / float64(255*480*(x1-x0))
	}
	if west, east := cover(0, 280), cover(360, 640); west < 0.1 || east > 0.01 {
		t.Errorf("laid over the screen the clouds cover %.2f of the west and %.2f of the east, "+
			"want the west alone", west, east)
	}
}

func TestRenderer_SetPutsItsOwnOverTheFramesWithoutAllocating(t *testing.T) {
	r := Renderer{uniforms: map[string]any{}}
	r.opts.Uniforms = map[string]any{}
	r.set("AloftView", 1, 2)
	r.opts.Uniforms["AloftView"] = []float32{9, 9}
	n := testing.AllocsPerRun(10, func() { r.set("AloftView", 3, 4) })
	if got := r.opts.Uniforms["AloftView"].([]float32); n != 0 || got[0] != 3 || got[1] != 4 {
		t.Errorf("AloftView set over another: %v allocations, %v; want none and [3 4]", n, got)
	}
}
