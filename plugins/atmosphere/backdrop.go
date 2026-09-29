package atmosphere

import (
	"embed"
	"image/color"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*Backdrop)(nil)

//go:embed shaders/*.wgsl
var shaders embed.FS

// skyShader draws the sky through a perspective (shaders/backdrop.wgsl), on the composer's library
// and its materials, the clouds among them.
var skyShader = render.NewMeshShaderWith("sky", render.Files(shaders, "shaders/backdrop.wgsl"), []render.Uniform{
	{Name: "SkyView", Size: 2}, {Name: "SkyHorizon", Size: 3}, {Name: "SkyOverhead", Size: 3},
	{Name: "SunAt", Size: 2}, {Name: "SunRadius", Size: 1}, {Name: "SunDisc", Size: 4}, {Name: "CloudsOn", Size: 1},
})

// Backdrop is the sky behind the world, behind everything, whenever the ground does not cover all
// of the viewport — beyond the world's edge, above a low view; a view the ground covers draws
// none. It is a render.Direct source at the Backdrop tier, drawn on the GPU before anything else.
// Through a camera with an eye that says every line of sight at once (camera.Eyed and camera.Rays,
// a perspective's) it is the sky of the day, every pixel looking along its own line of sight:
// paler at the horizon, deeper overhead, greyed as much as the clouds cover it, the sun standing
// in it where the way towards it vanishes, and the clouds themselves on a layer air.Base high — at
// the same clouds, by the same noise, that lay their shadows straight under them on the ground —
// hazed away towards the horizon. Any other camera gets the viewport filled in the sky's colour.
type Backdrop struct {
	space world.SpaceCfg
	scale world.Scale
	sun   func() sky.Sun
	air   func() air.Weather

	tri      *render.Indices // the one triangle over the viewport
	opts     render.DrawMeshOptions
	uniforms map[string][]float32 // the sky's own, boxed once in the options
}

// skyPlan is what the backdrop draws through a camera: nothing where the ground covers the
// viewport; the viewport Flat in the horizon's colour through a camera without an eye, else the
// sky of the rays Field from the Eye, View pixels, from Horizon to Overhead; the sun's disc
// SunRadius pixels wide round SunAt in SunDisc (none at 0); and the clouds CloudHeight up, seen as
// far as Seeing, where Clouds says.
type skyPlan struct {
	None, Flat          bool
	Horizon, Overhead   render.Light
	View                [2]float32
	Field               camera.RayField
	Eye                 [3]float32
	SunAt               [2]float32
	SunRadius           float32
	SunDisc             [4]float32
	Clouds              bool
	CloudHeight, Seeing float32
}

// NewBackdrop is the sky behind a world of space and scale, under sun and the weather air give.
func NewBackdrop(space world.SpaceCfg, scale world.Scale, sun func() sky.Sun, weather func() air.Weather) *Backdrop {
	b := &Backdrop{space: space, scale: scale, sun: sun, air: weather, uniforms: map[string][]float32{}}
	b.opts.Uniforms = map[string]any{}
	return b
}

func (*Backdrop) Init(*goke.SysInit) {}

// Compose hands the frame nothing: the sky is drawn Direct.
func (*Backdrop) Compose(*render.Frame, camera.Camera) {}

// Tier is where the sky comes in the picture: first, render.Backdrop.
func (*Backdrop) Tier() render.Tier { return render.Backdrop }

// Draw draws the sky into the target's screen through cam under the frame's uniforms u.
func (b *Backdrop) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	p := b.plan(cam)
	if t.Screen == nil || p.None {
		return
	}
	if p.Flat && p.SunRadius == 0 {
		t.Screen.Fill(rgba(p.Horizon))
		return
	}
	if b.tri == nil {
		b.tri = render.NewIndices([]uint32{0, 1, 2})
	}
	u.Into(b.opts.Uniforms)
	f := p.Field
	b.set("SkyView", p.View[:]...)
	b.set("SkyHorizon", p.Horizon[:]...)
	b.set("SkyOverhead", p.Overhead[:]...)
	b.set("SunAt", p.SunAt[:]...)
	b.set("SunRadius", p.SunRadius)
	b.set("SunDisc", p.SunDisc[:]...)
	b.set("CloudsOn", flag(p.Clouds))
	b.set("EyeAt", p.Eye[:]...)
	b.set("LookDir", f.Dir[:]...)
	b.set("LookDX", f.DDX[:]...)
	b.set("LookDY", f.DDY[:]...)
	b.set("CloudHeight", p.CloudHeight)
	b.set("Visibility", p.Seeing)
	t.Screen.DrawMesh(b.tri, skyShader, &b.opts)
}

// set hands the sky's shader the uniform name as v, in a slice kept between frames and boxed once.
func (b *Backdrop) set(name string, v ...float32) {
	s, ok := b.uniforms[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		b.uniforms[name] = s
	}
	copy(s, v)
	b.opts.Uniforms[name] = s
}

func flag(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

// plan is what the sky is through cam this frame.
func (b *Backdrop) plan(cam camera.Camera) skyPlan {
	w, h := cam.Viewport()
	if b.covered(cam, w, h) {
		return skyPlan{None: true}
	}
	day := b.sun()
	weather := b.air()
	p := skyPlan{Horizon: air.Overcast(day.SkyLight(), weather.Clouds), View: [2]float32{w, h}}
	p.SunAt, p.SunRadius, p.SunDisc = b.sunOn(cam, w, h, day, weather.Clouds)
	field, ex, ey, ez, eyed := lines(cam)
	if !eyed {
		p.Flat, p.Overhead = true, p.Horizon
		return p
	}
	p.Field, p.Eye = field, [3]float32{ex, ey, ez}
	p.Overhead = air.Overhead(day.SkyLight(), weather.Clouds)
	p.CloudHeight = air.Base(b.scale)
	p.Clouds = weather.Clouds > 0 && ez < p.CloudHeight
	p.Seeing = float32(air.Visibility(b.scale, weather))
	return p
}

// lines is cam's lines of sight from its eye — a perspective's, whose rays all start at the eye —
// and the eye; false for any other camera.
func lines(cam camera.Camera) (field camera.RayField, ex, ey, ez float32, ok bool) {
	e, eyed := cam.(camera.Eyed)
	r, rays := cam.(camera.Rays)
	if !eyed || !rays {
		return field, 0, 0, 0, false
	}
	ex, ey, ez, ok = e.Eye()
	if !ok {
		return field, 0, 0, 0, false
	}
	field, ok = r.Rays()
	return field, ex, ey, ez, ok && field.DX == [3]float32{} && field.DY == [3]float32{}
}

func rgba(l render.Light) color.RGBA {
	return color.RGBA{R: channel(l[0]), G: channel(l[1]), B: channel(l[2]), A: 255}
}

// covered reports whether the ground covers the w x h viewport: a wrapping world seen through a
// projection that wraps, or every corner of the screen over the world.
func (b *Backdrop) covered(cam camera.Camera, w, h float32) bool {
	space := b.space
	if space.Edges.WrapsX() && space.Edges.WrapsY() && cam.Projection().Wraps() {
		return true
	}
	ww, wh := float32(space.Width), float32(space.Height)
	for _, p := range [4][2]float32{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		x, y := cam.Unproject(p[0], p[1], 0)
		if x < 0 || y < 0 || x > ww || y > wh {
			return false
		}
	}
	return true
}

// The sun's disc as a part of the screen's height, and how many times wider its glow is (the
// shader's sunGlow, the same).
const (
	sunRadius = 1.0 / 40
	sunGlow   = 3
)

// sunOn is where the sun's disc stands on the screen — where the way towards it vanishes, through
// a camera with vanishing points (camera.Vanisher), a perspective's — how wide it is and its colour,
// over the horizon, dimmed by the clouds; the world draws over it, so it sets behind the hills,
// and the clouds over it. Its radius is 0 where none shows.
func (b *Backdrop) sunOn(cam camera.Camera, w, h float32, day sky.Sun, clouds float32) (at [2]float32, radius float32, disc [4]float32) {
	dir := day.Dir
	v, ok := cam.(camera.Vanisher)
	if !ok || dir[2] <= 0 || day.Strength <= 0 {
		return at, 0, disc
	}
	ax, ay, ahead := v.Vanish(dir[0], dir[1], dir[2])
	if !ahead {
		return at, 0, disc
	}
	r := h * sunRadius
	if ax < -sunGlow*r || ax > w+sunGlow*r || ay < -sunGlow*r || ay > h+sunGlow*r {
		return at, 0, disc
	}
	clear := 1 - min(max(clouds, 0), 1)
	if clear <= 0 {
		return at, 0, disc
	}
	light := day.Color
	if light == (render.Light{}) {
		light = render.Light{1, 1, 1}
	}
	return [2]float32{ax, ay}, r, [4]float32{0.5 + 0.5*light[0], 0.5 + 0.5*light[1], 0.5 + 0.5*light[2], clear}
}

func channel(v float32) uint8 { return uint8(min(max(v, 0), 1)*255 + 0.5) }
