package precipitation

import (
	"embed"
	"image/color"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*Renderer)(nil)

//go:embed shaders/*.wgsl
var shaders embed.FS

var shader = render.NewMeshShaderWith("precipitation", render.Files(shaders, "shaders/fall.wgsl"), []render.Uniform{
	{Name: "FallView", Size: 2}, {Name: "FallDrift", Size: 1}, {Name: "Drops", Size: 1}, {Name: "Flakes", Size: 1},
	{Name: "RainColor", Size: 4}, {Name: "SnowColor", Size: 4},
})

// Renderer is what falls through the air before the eye, rain and snow, a render.Direct before
// the whole world.
type Renderer struct {
	sun func() sky.Sun
	air func() air.Weather

	opts     render.DrawMeshOptions
	uniforms map[string][]float32 // boxed once in the options
}

// New is what falls under sun out of the weather air gives.
func New(sun func() sky.Sun, weather func() air.Weather) *Renderer {
	r := &Renderer{sun: sun, air: weather, uniforms: map[string][]float32{}}
	r.opts.Uniforms = map[string]any{}
	return r
}

// pixels of screen a drop takes, pixels a second, pixels long and wide; the shader has the same
const (
	rainRoom  = 900
	snowRoom  = 1400
	rainSpeed = 700
	snowSpeed = 55
	rainDrop  = 18
	snowFlake = 3
	// drops fall a long way before the eye, so the wind slants them more than it carries them
	rainSlant = 5
)

// Drift in pixels a second; the colours premultiplied
type fall struct {
	Drops, Flakes int
	View          [2]float32
	Drift         float32
	Rain, Snow    [4]float32
}

func (*Renderer) Init(*goke.SysInit) {}

// Compose has what falls drawn Direct, before the whole world.
func (*Renderer) Compose(f *render.Frame, _ camera.Camera) { f.DrawAt(render.Nearest) }

// Draw draws what falls into the target's screen through cam.
func (p *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	f := p.fall(cam)
	if t.Screen == nil || f.Drops+f.Flakes == 0 {
		return
	}
	u.Into(p.opts.Uniforms)
	p.set("FallView", f.View[:]...)
	p.set("FallDrift", f.Drift)
	p.set("Drops", float32(f.Drops))
	p.set("Flakes", float32(f.Flakes))
	p.set("RainColor", f.Rain[:]...)
	p.set("SnowColor", f.Snow[:]...)
	p.opts.Vertices = 6 * (f.Drops + f.Flakes)
	p.opts.Depth, p.opts.Untested = t.Depth, true
	t.Screen.DrawMesh(nil, shader, &p.opts)
}

func (p *Renderer) set(name string, v ...float32) {
	s, ok := p.uniforms[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		p.uniforms[name] = s
	}
	copy(s, v)
	p.opts.Uniforms[name] = s
}

func (p *Renderer) fall(cam camera.Camera) fall {
	air := p.air()
	if air.Rain < 0.02 && air.Snow < 0.02 {
		return fall{}
	}
	w, h := cam.Viewport()
	light := p.sun().Light(0, 0, 1)
	// measured where the middle of the screen looks: through a perspective a point elsewhere may
	// lie behind the eye
	mx, my := cam.Unproject(w/2, h/2, 0)
	x0, _ := cam.Project(mx, my, 0)
	x1, _ := cam.Project(mx+air.Wind[0], my+air.Wind[1], 0)
	return fall{
		Drops:  max(int(air.Rain*w*h/rainRoom), 0),
		Flakes: max(int(air.Snow*w*h/snowRoom), 0),
		View:   [2]float32{w, h},
		Drift:  min(max((x1-x0)*rainSlant, -rainSpeed), rainSpeed),
		Rain:   shade(color.RGBA{R: 185, G: 195, B: 215, A: 150}, light),
		Snow:   shade(color.RGBA{R: 240, G: 245, B: 255, A: 230}, light),
	}
}

// premultiplied, 0 to 1 a channel
func shade(c color.RGBA, light render.Light) [4]float32 {
	scale := func(v uint8, l float32) float32 {
		return float32(uint8(float32(v)*float32(c.A)/255*min(max(l, 0), 1))) / 255
	}
	return [4]float32{scale(c.R, light[0]), scale(c.G, light[1]), scale(c.B, light[2]), float32(c.A) / 255}
}
