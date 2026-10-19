package precipitation

import (
	"embed"
	"image/color"
	"math"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*Drops)(nil)
var _ render.Overlying = (*Drops)(nil)

//go:embed shaders/*.wgsl
var shaders embed.FS

var dropShader = render.NewMeshShaderWith("drops", render.Files(shaders, "shaders/drops.wgsl"),
	[]render.Uniform{
		{Name: "DropViewProj", Size: 16},
		{Name: "DropEye", Size: 3},
		{Name: "DropBend", Size: 1},
		{Name: "DropView", Size: 2},
		{Name: "DropPx", Size: 1},
		{Name: "DropOrtho", Size: 1},
		{Name: "DropBox0", Size: 4},
		{Name: "DropBox1", Size: 4},
		{Name: "DropBox2", Size: 4},
		{Name: "DropTall", Size: 3},
		{Name: "DropsEach", Size: 1},
		{Name: "DropLean", Size: 2},
		{Name: "DropSpeed", Size: 1},
		{Name: "DropLength", Size: 1},
		{Name: "DropWidth", Size: 1},
		{Name: "DropColor", Size: 4},
	})

// Drops is what falls, drop by drop, between the low clouds and the ground, seen through a camera
// as it would be: a dot from above, a short line from the side, nothing too far off to make out;
// a render.Direct among what is seen through.
type Drops struct {
	sun func() light.Distant
	air func() air.Weather

	opts     render.DrawMeshOptions
	uniforms map[string][]float32 // boxed once in the options
}

// NewDrops is what falls out of the weather air gives, drop by drop, under sun.
func NewDrops(sun func() light.Distant, weather func() air.Weather) *Drops {
	d := &Drops{sun: sun, air: weather, uniforms: map[string][]float32{}}
	d.opts.Uniforms = map[string]any{}
	return d
}

// the falls under which nothing is drawn
const dry = 0.02

// how many drops a box holds; a raindrop's streak and a flake, world units long, how fast they
// fall, world units a second, how wide, pixels; how far the wind leans them, across for every
// world unit down per world unit a second of wind, and at most, rain and snow; how many pixels
// long a drop must stand to show at all (the shaders have the same)
const (
	dropsEach  = 12000
	rainLength = 0.5
	rainSpeed  = 18
	rainWidth  = 1.4
	snowLength = 0.15
	snowSpeed  = 2.5
	snowWidth  = 2
	dropLean   = 0.01
	rainLeans  = 0.25
	snowLeans  = 0.6
	dropLeast  = 0.75
)

// a drop's look in the weather w: how long, how fast, how wide, how far the wind leans it, its
// colour as yet unlit
type dropLook struct {
	length, speed, width float32
	lean                 [2]float32
	color                color.RGBA
}

func lookOf(w air.Weather) dropLook {
	l := dropLook{length: rainLength, speed: rainSpeed, width: rainWidth,
		color: color.RGBA{R: 215, G: 225, B: 240, A: 255}}
	most := float32(rainLeans)
	if _, snow := w.Falls(air.Mass{Falls: 1}); snow > 0 {
		l = dropLook{length: snowLength, speed: snowSpeed, width: snowWidth,
			color: color.RGBA{R: 245, G: 248, B: 255, A: 255}}
		most = snowLeans
	}
	l.lean = [2]float32{w.Wind[0] * dropLean, w.Wind[1] * dropLean}
	if n := float32(math.Hypot(float64(l.lean[0]), float64(l.lean[1]))); n > most {
		l.lean = [2]float32{l.lean[0] * most / n, l.lean[1] * most / n}
	}
	return l
}

func (*Drops) Init(*goke.SysInit) {}

// Overlies: drawn over the figures' ground (render.Overlying).
func (*Drops) Overlies() {}

// Compose has the drops drawn among what is seen through, half way down from the low clouds where
// the eye looks, while anything falls out of them.
func (d *Drops) Compose(f *render.Frame, cam camera.Camera) {
	b, ok := d.boxes(cam)
	if !ok {
		return
	}
	f.DrawAt(f.At(b.middle[0], b.middle[1], d.air().Base(air.Low)/2))
}

// where the drops are drawn through a camera: three boxes round the middle, each its width and
// height, none where 0; how many pixels a world unit spans, a world unit off through a perspective
type boxes struct {
	middle [3]float32
	wide   [3]float32
	tall   [3]float32
	px     float32
	ortho  bool
	field  camera.RayField
}

func (d *Drops) boxes(cam camera.Camera) (boxes, bool) {
	w := d.air()
	if most := w.Most(); most.Falls < dry || w.Share(most, air.Low).Cover <= 0 {
		return boxes{}, false
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return boxes{}, false
	}
	f, ok := rays.Rays()
	if !ok {
		return boxes{}, false
	}
	_, base := w.BaseSpan(air.Low) // the highest, any drop under it
	length := lookOf(w).length
	vw, vh := cam.Viewport()
	b := boxes{field: f, ortho: f.DDX == ([3]float32{}) && f.DDY == ([3]float32{})}
	if b.ortho {
		b.px = 1 / norm(f.DX)
		if length*b.px < dropLeast {
			return boxes{}, false // too small to make out from so far
		}
		// round where the middle of the view looks half way down from the clouds, as wide as the
		// view sees the air between the clouds and the sea
		at := func(sx, sy, z float32) [3]float32 {
			o := add(f.Origin, add(scale(f.DX, sx), scale(f.DY, sy)))
			return add(o, scale(f.Dir, (z-o[2])/f.Dir[2]))
		}
		if f.Dir[2] == 0 {
			return boxes{}, false
		}
		b.middle = at(vw/2, vh/2, base/2)
		lo, hi := b.middle, b.middle
		for _, c := range [4][2]float32{{0, 0}, {vw, 0}, {0, vh}, {vw, vh}} {
			for _, z := range []float32{0, base} {
				p := at(c[0], c[1], z)
				for k := range 2 {
					lo[k], hi[k] = min(lo[k], p[k]), max(hi[k], p[k])
				}
			}
		}
		b.wide[0] = max(hi[0]-lo[0], hi[1]-lo[1], base)
		b.tall[0] = base // all of the air under the clouds, seen through from above
		return b, true
	}
	b.px = 1 / norm(f.DDY)
	reach := length * b.px / dropLeast // past it a drop is too small to make out
	eye := f.Origin
	b.middle = eye
	if eye[2] >= base || eye[2] <= 0 {
		// from over the clouds or under the sea, where the view's middle meets their half way
		way := add(f.Dir, add(scale(f.DDX, vw/2), scale(f.DDY, vh/2)))
		if way[2] == 0 {
			return boxes{}, false
		}
		t := (base/2 - eye[2]) / way[2]
		if t <= 0 {
			return boxes{}, false
		}
		b.middle = add(eye, scale(way, t))
		if dist := norm(add(b.middle, scale(eye, -1))); dist > reach {
			return boxes{}, false
		}
	}
	b.wide = [3]float32{reach / 8, reach / 2, 2 * reach}
	b.tall = b.wide
	return b, true
}

// Draw draws the drops into the target's screen through cam, hidden behind what is drawn nearer.
func (d *Drops) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	b, ok := d.boxes(cam)
	if t.Screen == nil || !ok {
		return
	}
	vw, vh := cam.Viewport()
	tr, ok := camera.SceneTransform(b.field, vw, vh)
	if !ok {
		return
	}
	weather := d.air()
	look := lookOf(weather)
	u.Into(d.opts.Uniforms)
	d.set("DropViewProj", tr.M[:]...)
	d.set("DropEye", tr.Eye[:]...)
	d.set("DropBend", tr.Bend)
	d.set("DropView", vw, vh)
	d.set("DropPx", b.px)
	d.set("DropOrtho", flag(b.ortho))
	each := dropsEach
	if b.ortho {
		each *= 3 // one box, all of them
	}
	for k, name := range []string{"DropBox0", "DropBox1", "DropBox2"} {
		d.set(name, b.middle[0], b.middle[1], b.middle[2], b.wide[k])
	}
	d.set("DropTall", b.tall[:]...)
	d.set("DropsEach", float32(each))
	d.set("DropLean", look.lean[:]...)
	d.set("DropSpeed", look.speed)
	d.set("DropLength", look.length)
	d.set("DropWidth", look.width)
	lit := shade(look.color, d.sun().Light(0, 0, 1))
	d.set("DropColor", lit[:]...)
	d.opts.Vertices = 6 * 3 * dropsEach
	d.opts.Depth = t.Depth
	t.Screen.DrawMesh(nil, dropShader, &d.opts)
}

func (d *Drops) set(name string, v ...float32) {
	s, ok := d.uniforms[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		d.uniforms[name] = s
	}
	copy(s, v)
	d.opts.Uniforms[name] = s
}

// premultiplied, 0 to 1 a channel
func shade(c color.RGBA, light render.Light) [4]float32 {
	scale := func(v uint8, l float32) float32 {
		return float32(uint8(float32(v)*float32(c.A)/255*min(max(l, 0), 1))) / 255
	}
	return [4]float32{scale(c.R, light[0]), scale(c.G, light[1]), scale(c.B, light[2]),
		float32(c.A) / 255}
}

func flag(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

func norm(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

func add(a, b [3]float32) [3]float32 { return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }

func scale(a [3]float32, k float32) [3]float32 { return [3]float32{a[0] * k, a[1] * k, a[2] * k} }
