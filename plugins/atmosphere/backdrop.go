package atmosphere

import (
	"image/color"
	"math"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var _ render.Source = (*Backdrop)(nil)

// Backdrop is the sky behind the world: the viewport filled in the colour of the world's sky, grey
// as much as the clouds cover it, behind everything, whenever the ground does not cover all of it
// — beyond the world's edge, above a low view; a view the ground covers draws none. Through a
// camera with vanishing points (camera.Vanisher) the sun stands in it, over the horizon, where the
// way towards it vanishes.
type Backdrop struct {
	space world.SpaceCfg
	sun   func() sky.Sun
	air   func() air.Weather
	pts   [][2]float32
}

// NewBackdrop is the sky behind a world of space, under sun and the weather air give.
func NewBackdrop(space world.SpaceCfg, sun func() sky.Sun, weather func() air.Weather) *Backdrop {
	return &Backdrop{space: space, sun: sun, air: weather}
}

func (*Backdrop) Init(*goke.SysInit) {}

func (b *Backdrop) Compose(f *render.Frame, cam camera.Camera) {
	w, h := cam.Viewport()
	if b.covered(cam, w, h) {
		return
	}
	day := b.sun()
	clouds := b.air().Clouds
	sky := air.Overcast(day.SkyLight(), clouds)
	c := color.RGBA{A: 255}
	c.R, c.G, c.B = channel(sky[0]), channel(sky[1]), channel(sky[2])
	f.Soft(render.Backdrop, float32(math.Inf(-1)), render.Corners{{0, 0}, {w, 0}, {0, h}, {w, h}}, c, render.Fade{})
	b.drawSun(f, cam, w, h, day, clouds)
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

// The sun's disc as a part of the screen's height, how many times wider its glow is, and how many
// points round its disc.
const (
	sunRadius = 1.0 / 40
	sunGlow   = 3
	sunPoints = 24
)

// sun draws the sun's disc and its glow where the way towards it vanishes on the screen — through
// a camera with vanishing points (camera.Vanisher), a perspective's — over the horizon, dimmed by
// the clouds; the world draws over it, so it sets behind the hills.
func (b *Backdrop) drawSun(f *render.Frame, cam camera.Camera, w, h float32, day sky.Sun, clouds float32) {
	dir := day.Dir
	v, ok := cam.(camera.Vanisher)
	if !ok || dir[2] <= 0 || day.Strength <= 0 {
		return
	}
	ax, ay, ahead := v.Vanish(dir[0], dir[1], dir[2])
	if !ahead {
		return
	}
	r := h * sunRadius
	if ax < -sunGlow*r || ax > w+sunGlow*r || ay < -sunGlow*r || ay > h+sunGlow*r {
		return
	}
	clear := 1 - min(max(clouds, 0), 1)
	if clear <= 0 {
		return
	}
	light := day.Color
	if light == (render.Light{}) {
		light = render.Light{1, 1, 1}
	}
	c := color.RGBA{R: channel(0.5 + 0.5*light[0]), G: channel(0.5 + 0.5*light[1]), B: channel(0.5 + 0.5*light[2]), A: channel(clear)}
	glow := c
	glow.A = channel(0.25 * clear)
	f.Fan(render.Backdrop, -math.MaxFloat32, b.disc(ax, ay, sunGlow*r), glow)
	f.Fan(render.Backdrop, -math.MaxFloat32, b.disc(ax, ay, r), c)
}

// disc is the polygon of a disc r wide round (x, y), from its middle.
func (b *Backdrop) disc(x, y, r float32) [][2]float32 {
	b.pts = append(b.pts[:0], [2]float32{x, y})
	for i := 0; i <= sunPoints; i++ {
		s, c := math.Sincos(2 * math.Pi * float64(i) / sunPoints)
		b.pts = append(b.pts, [2]float32{x + r*float32(c), y + r*float32(s)})
	}
	return b.pts
}

func channel(v float32) uint8 { return uint8(min(max(v, 0), 1)*255 + 0.5) }
