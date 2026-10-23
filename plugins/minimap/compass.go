package minimap

import (
	"image/color"
	"math"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

var northColor = color.RGBA{R: 230, G: 40, B: 40, A: 255}

// Overhang is how far past the minimap's circle its compass reaches, the pixels a scene grows
// the box it lays the compass over on every side.
const Overhang = 20

// the triangle's height and width, and how much of its height stands over the circle's edge
const (
	northTall = 18
	northWide = 16
	northOut  = 0.75
)

// Compass is a red triangle hung on the edge of cam's minimap pointing north, three quarters
// of it over the edge, turned round the circle and with it as the minimap turns: a picture for
// the scene to lay over the minimap in a box Overhang pixels bigger than the minimap's on every
// side (ui.Layers of the minimap centred at its size and the compass), transparent elsewhere.
// Nothing for a camera without a minimap of the plugin's.
func (p *Plugin) Compass(cam camera.Camera) *ui.Element {
	for _, m := range p.maps {
		if m.of == cam {
			return ui.Image(&compass{m: m})
		}
	}
	return ui.Blank()
}

// compass draws the triangle over a transparent picture of the box's size
type compass struct {
	m    *minimap
	w, h int
	img  *render.Image
}

var _ render.Surface = (*compass)(nil)

func (c *compass) Resize(w, h int) { c.w, c.h = w, h }

func (c *compass) Draw() *render.Image {
	if c.w <= 0 || c.h <= 0 {
		return nil
	}
	if c.img == nil || c.img.Bounds().Dx() != c.w || c.img.Bounds().Dy() != c.h {
		if c.img != nil {
			c.img.Deallocate()
		}
		c.img = render.NewImage(c.w, c.h)
	}
	c.img.Clear()
	c.img.FillPolygon(c.points(), northColor)
	return c.img
}

// north is the way north runs on the picture, a unit vector: (0, -1) in the world, on a
// picture whose up is the lens's ahead (ax, ay), right ahead turned a quarter clockwise
func (c *compass) north() (float64, float64) {
	l := c.m.lens
	dx, dy := -float64(l.ax), float64(l.ay)
	if n := math.Hypot(dx, dy); n > 0 {
		return dx / n, dy / n
	}
	return 0, -1
}

// at is the triangle's middle in the box: on the way north from the box's middle, the circle's
// radius and the triangle's quarter out
func (c *compass) at() (float32, float32) {
	dx, dy := c.north()
	radius := float64(min(c.w, c.h))/2 - Overhang
	d := radius + northTall*(northOut-0.5)
	return float32(float64(c.w)/2 + dx*d), float32(float64(c.h)/2 + dy*d)
}

// points is the triangle: its tip north of its middle, its base across
func (c *compass) points() [][2]float32 {
	dx, dy := c.north()
	mx, my := c.at()
	tip := [2]float32{mx + float32(dx*northTall/2), my + float32(dy*northTall/2)}
	bx, by := mx-float32(dx*northTall/2), my-float32(dy*northTall/2)
	// across is north turned a quarter
	ax, ay := float32(-dy*northWide/2), float32(dx*northWide/2)
	return [][2]float32{tip, {bx + ax, by + ay}, {bx - ax, by - ay}}
}
