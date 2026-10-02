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

// shader draws what falls (shaders/fall.wgsl), every drop worked out on the GPU from its number.
var shader = render.NewMeshShaderWith("precipitation", render.Files(shaders, "shaders/fall.wgsl"), []render.Uniform{
	{Name: "FallView", Size: 2}, {Name: "FallDrift", Size: 1}, {Name: "Drops", Size: 1}, {Name: "Flakes", Size: 1},
	{Name: "RainColor", Size: 4}, {Name: "SnowColor", Size: 4},
})

// Renderer is what falls through the air before the eye: rain as streaks slanting with the wind,
// snow as flakes drifting down, as many as the weather says, in the light of the sun — a
// render.Direct source at render.Air, drawn on the GPU. Each drop is where its number and the
// frame's time put it, so nothing is kept between frames; with nothing falling, nothing is drawn.
type Renderer struct {
	sun func() sky.Sun
	air func() air.Weather

	opts     render.DrawMeshOptions
	uniforms map[string][]float32 // its own, boxed once in the options
}

// New is what falls under sun out of the weather air gives.
func New(sun func() sky.Sun, weather func() air.Weather) *Renderer {
	r := &Renderer{sun: sun, air: weather, uniforms: map[string][]float32{}}
	r.opts.Uniforms = map[string]any{}
	return r
}

// How it falls: pixels of screen a drop takes at full rain, a flake at full snow; how fast each
// falls, pixels a second; how long a streak of rain is and how wide a flake (the shader's, the
// same).
const (
	rainRoom  = 900
	snowRoom  = 1400
	rainSpeed = 700
	snowSpeed = 55
	rainDrop  = 18
	snowFlake = 3
	// rainSlant is how much more the wind slants what falls than it carries it: drops fall a long
	// way before the eye
	rainSlant = 5
)

// fall is what falls through cam this frame: Drops of rain and Flakes of snow over a screen View
// pixels wide, the wind carrying them Drift pixels a second across it, in the colours Rain and
// Snow, premultiplied.
type fall struct {
	Drops, Flakes int
	View          [2]float32
	Drift         float32
	Rain, Snow    [4]float32
}

func (*Renderer) Init(*goke.SysInit) {}

// Compose hands the frame nothing: what falls is drawn Direct.
func (*Renderer) Compose(*render.Frame, camera.Camera) {}

// Tier is where what falls comes in the picture: render.Air, over the world, under the marks.
func (*Renderer) Tier() render.Tier { return render.Air }

// Draw draws what falls into the target's screen through cam at the frame's clock in u.
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
	t.Screen.DrawMesh(nil, shader, &p.opts)
}

// set hands the shader the uniform name as v, in a slice kept between frames and boxed once.
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
	// how far the wind carries a drop across the screen in a second, where the middle of the screen
	// looks — through a perspective a point elsewhere may lie behind the eye — no more than it
	// falls
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

// shade is c, premultiplied, dimmed by the light it is seen in, 0 to 1 a channel.
func shade(c color.RGBA, light render.Light) [4]float32 {
	scale := func(v uint8, l float32) float32 {
		return float32(uint8(float32(v)*float32(c.A)/255*min(max(l, 0), 1))) / 255
	}
	return [4]float32{scale(c.R, light[0]), scale(c.G, light[1]), scale(c.B, light[2]), float32(c.A) / 255}
}
