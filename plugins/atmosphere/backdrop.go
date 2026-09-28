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

// Backdrop is the sky behind the world, behind everything, whenever the ground does not cover all
// of the viewport — beyond the world's edge, above a low view; a view the ground covers draws
// none. Through a camera that says which way each screen point looks (camera.Rayer, a
// perspective's) it is the sky of the day: paler at the horizon, deeper overhead, greyed as much
// as the clouds cover it, the sun standing in it where the way towards it vanishes, and the
// clouds themselves drawn on a layer air.Base high — the same clouds, by the same noise, that lay
// their shadows straight under them on the ground — hazed away towards the horizon. Any other
// camera gets the viewport filled in the sky's colour.
type Backdrop struct {
	space world.SpaceCfg
	scale world.Scale
	sun   func() sky.Sun
	air   func() air.Weather
	pts   [][2]float32
	mesh  []skyCorner // the corners of the sky's mesh, row by row, kept between frames
}

// skyCorner is one corner of the sky's mesh: where it lies on the screen, which way it looks
// (a unit vector, its rise dz), and, where it looks at the cloud layer, the point of it and the
// clouds' noise and haze there.
type skyCorner struct {
	sx, sy     float32
	dx, dy, dz float32
	x, y       float32
	n, haze    float32
	clouded    bool
}

// skyPiece is how many pixels a piece of the sky's mesh spans; skyLift is how far up a ray must
// look to reach the cloud layer at all, so the pieces along the horizon show haze alone.
const (
	skyPiece = 64
	skyLift  = 0.02
)

// NewBackdrop is the sky behind a world of space and scale, under sun and the weather air give.
func NewBackdrop(space world.SpaceCfg, scale world.Scale, sun func() sky.Sun, weather func() air.Weather) *Backdrop {
	return &Backdrop{space: space, scale: scale, sun: sun, air: weather}
}

func (*Backdrop) Init(*goke.SysInit) {}

func (b *Backdrop) Compose(f *render.Frame, cam camera.Camera) {
	w, h := cam.Viewport()
	if b.covered(cam, w, h) {
		return
	}
	day := b.sun()
	weather := b.air()
	horizon := air.Overcast(day.SkyLight(), weather.Clouds)
	rayer, rays := cam.(camera.Rayer)
	if rays {
		if _, _, _, ok := rayer.Ray(w/2, h/2); !ok {
			rays = false
		}
	}
	if !rays {
		f.Soft(render.Backdrop, float32(math.Inf(-1)), render.Corners{{0, 0}, {w, 0}, {0, h}, {w, h}}, rgba(horizon), render.Fade{})
		b.drawSun(f, cam, w, h, day, weather.Clouds)
		return
	}
	overhead := air.Overhead(day.SkyLight(), weather.Clouds)
	nx, ny := b.meshOf(cam, rayer, w, h, weather)
	for j := 0; j < ny; j++ {
		for i := 0; i < nx; i++ {
			var dst render.Corners
			var c [4]color.RGBA
			for k, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				at := &b.mesh[(j+d[1])*(nx+1)+i+d[0]]
				dst[k] = [2]float32{at.sx, at.sy}
				c[k] = rgba(between(horizon, overhead, at.dz))
			}
			f.Quad(render.Backdrop, float32(math.Inf(-1)), dst, c)
		}
	}
	b.drawSun(f, cam, w, h, day, weather.Clouds)
	b.drawClouds(f, nx, ny, weather)
}

// meshOf lays the sky's mesh over the w x h screen: a corner every skyPiece pixels, each with the
// way it looks and, for an eye under the cloud layer, the point of the layer it looks at and the
// clouds there; it reports the mesh's pieces across and down.
func (b *Backdrop) meshOf(cam camera.Camera, rayer camera.Rayer, w, h float32, weather air.Weather) (nx, ny int) {
	nx, ny = int(math.Ceil(float64(w/skyPiece))), int(math.Ceil(float64(h/skyPiece)))
	base := air.Base(b.scale)
	var ex, ey, ez float32
	under := false
	if e, ok := cam.(camera.Eyed); ok {
		var has bool
		ex, ey, ez, has = e.Eye()
		under = has && ez < base
	}
	b.mesh = b.mesh[:0]
	for j := 0; j <= ny; j++ {
		for i := 0; i <= nx; i++ {
			c := skyCorner{sx: min(float32(i)*skyPiece, w), sy: min(float32(j)*skyPiece, h)}
			c.dx, c.dy, c.dz, _ = rayer.Ray(c.sx, c.sy)
			if under && c.dz > skyLift {
				t := (base - ez) / c.dz
				c.x, c.y, c.clouded = ex+c.dx*t, ey+c.dy*t, true
				c.n = weather.Cloud(c.x, c.y)
				c.haze = weather.Haze(cam, c.x, c.y, base)
			}
			b.mesh = append(b.mesh, c)
		}
	}
	return nx, ny
}

// drawClouds lays the clouds on every piece of the mesh whose corners all look at the cloud layer,
// over the sun and under everything else.
func (b *Backdrop) drawClouds(f *render.Frame, nx, ny int, weather air.Weather) {
	if weather.Clouds <= 0 {
		return
	}
	for j := 0; j < ny; j++ {
		for i := 0; i < nx; i++ {
			var dst render.Corners
			var wo render.World
			var cloud, haze [4]float32
			all := true
			for k, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				at := &b.mesh[(j+d[1])*(nx+1)+i+d[0]]
				all = all && at.clouded
				dst[k], wo[k], cloud[k], haze[k] = [2]float32{at.sx, at.sy}, [2]float32{at.x, at.y}, at.n, at.haze
			}
			if all {
				weather.CloudQuad(f, render.Backdrop, -math.MaxFloat32/2, dst, wo, cloud, haze)
			}
		}
	}
}

// between is the sky's colour for a ray rising dz: the horizon's at 0 and below, the overhead's
// straight up, deepening quickly as the eye lifts.
func between(horizon, overhead render.Light, dz float32) render.Light {
	k := float32(math.Sqrt(float64(min(max(dz, 0), 1))))
	return render.Light{horizon[0] + (overhead[0]-horizon[0])*k, horizon[1] + (overhead[1]-horizon[1])*k, horizon[2] + (overhead[2]-horizon[2])*k}
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

// The sun's disc as a part of the screen's height, how many times wider its glow is, and how many
// points round its disc.
const (
	sunRadius = 1.0 / 40
	sunGlow   = 3
	sunPoints = 24
)

// sun draws the sun's disc and its glow where the way towards it vanishes on the screen — through
// a camera with vanishing points (camera.Vanisher), a perspective's — over the horizon, dimmed by
// the clouds; the world draws over it, so it sets behind the hills, and the clouds over it.
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
