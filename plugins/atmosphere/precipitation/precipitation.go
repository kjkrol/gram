package precipitation

import (
	"image/color"
	"math"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var _ render.Source = (*Renderer)(nil)

// Renderer is what falls through the air before the eye: rain as streaks slanting with the wind,
// snow as flakes drifting down, as many as the world's weather says, in the light of its sun, on
// render.Air. Each drop is where its number and the frame's time put it, so nothing is kept
// between frames; with nothing falling, nothing is drawn.
type Renderer struct{ world *world.Plugin }

// New is what falls over w.
func New(w *world.Plugin) *Renderer { return &Renderer{world: w} }

// How it falls: pixels of screen a drop takes at full rain, a flake at full snow; how fast each
// falls, pixels a second; how long a streak of rain is and how wide a flake.
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

func (*Renderer) Init(*goke.SysInit) {}

func (p *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	air := p.world.Weather()
	if air.Rain < 0.02 && air.Snow < 0.02 {
		return
	}
	w, h := cam.Viewport()
	light := p.world.Sun().Light(0, 0, 1)
	// how far the wind carries a drop across the screen in a second, where the middle of the screen
	// looks — through a perspective a point elsewhere may lie behind the eye — no more than it
	// falls
	mx, my := cam.Unproject(w/2, h/2, 0)
	x0, _ := cam.Project(mx, my, 0)
	x1, _ := cam.Project(mx+air.Wind[0], my+air.Wind[1], 0)
	drift := min(max((x1-x0)*rainSlant, -rainSpeed), rainSpeed)
	t := f.Time()
	depth := float32(math.Inf(1))

	rain := shade(color.RGBA{R: 185, G: 195, B: 215, A: 150}, light)
	for i := range int(air.Rain * w * h / rainRoom) {
		u, v := spot(i)
		y := wrapAround(v*h+t*rainSpeed, h+rainDrop) - rainDrop
		x := wrapAround(u*w+t*drift, w)
		f.Line(render.Air, depth, x, y, x+drift*rainDrop/rainSpeed, y+rainDrop, 1, rain)
	}
	snow := shade(color.RGBA{R: 240, G: 245, B: 255, A: 230}, light)
	for i := range int(air.Snow * w * h / snowRoom) {
		u, v := spot(i + 1<<20)
		y := wrapAround(v*h+t*snowSpeed*(0.7+0.6*u), h+snowFlake) - snowFlake
		x := wrapAround(u*w+t*drift*0.5+6*float32(math.Sin(float64(t*1.3+u*40))), w)
		f.Soft(render.Air, depth, render.Corners{{x, y}, {x + snowFlake, y}, {x, y + snowFlake}, {x + snowFlake, y + snowFlake}}, snow,
			render.Fade{Left: 1, Right: 1, Top: 1, Bottom: 1})
	}
}

// spot is where drop i starts, 0 to 1 across and down the screen, fixed for it.
func spot(i int) (float32, float32) {
	x := uint32(i)*2654435761 + 0x9e3779b9
	x ^= x >> 16
	x *= 0x85ebca6b
	x ^= x >> 13
	y := x*0xc2b2ae35 + 0x27d4eb2f
	y ^= y >> 15
	return float32(x&0xffff) / 0x10000, float32(y&0xffff) / 0x10000
}

// wrapAround is v within 0 up to n.
func wrapAround(v, n float32) float32 {
	v = float32(math.Mod(float64(v), float64(n)))
	if v < 0 {
		v += n
	}
	return v
}

// shade is c, premultiplied, dimmed by the light it is seen in.
func shade(c color.RGBA, light render.Light) color.RGBA {
	scale := func(v uint8, l float32) uint8 {
		return uint8(float32(v) * float32(c.A) / 255 * min(max(l, 0), 1))
	}
	return color.RGBA{R: scale(c.R, light[0]), G: scale(c.G, light[1]), B: scale(c.B, light[2]), A: c.A}
}
