package minimap

import (
	"image/color"
	"math"

	"golang.org/x/image/font/gofont/gobold"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

var northColor = color.RGBA{R: 230, G: 40, B: 40, A: 255}

// Overhang is how far past the minimap's circle its compass reaches, the pixels a scene grows
// the box it lays the compass over on every side.
const Overhang = 20

// how much of the letter stands over the circle's edge
const northOut = 0.75

var northFont = func() *render.Font {
	f, err := render.NewFont(gobold.TTF, 28)
	if err != nil {
		panic("minimap: the compass's font: " + err.Error())
	}
	return f
}()

// Compass is a red bold "N" hung on the edge of cam's minimap where north lies, three quarters
// of it over the edge, turned round the circle as the minimap turns: a picture for the scene to
// lay over the minimap in a box Overhang pixels bigger than the minimap's on every side
// (ui.Layers of the minimap centred at its size and the compass), transparent elsewhere.
// Nothing for a camera without a minimap of the plugin's.
func (p *Plugin) Compass(cam camera.Camera) *ui.Element {
	for _, m := range p.maps {
		if m.of == cam {
			return ui.Image(&compass{m: m})
		}
	}
	return ui.Blank()
}

// compass draws the letter over a transparent picture of the box's size
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
	x, y := c.at()
	w, h := northFont.Measure("N")
	render.DrawText(c.img, northFont, "N", x-w/2, y-h/2, northColor)
	return c.img
}

// at is the letter's middle in the box: on the way north from the box's middle, the circle's
// radius and the letter's quarter out
func (c *compass) at() (float32, float32) {
	l := c.m.lens
	// north, (0, -1) in the world, on the picture whose up is the lens's ahead (ax, ay): right
	// is ahead turned a quarter clockwise, as the lens projects
	dx, dy := -float64(l.ax), float64(l.ay)
	if n := math.Hypot(dx, dy); n > 0 {
		dx, dy = dx/n, dy/n
	} else {
		dx, dy = 0, -1
	}
	_, h := northFont.Measure("N")
	radius := float64(min(c.w, c.h))/2 - Overhang
	d := radius + float64(h)*(northOut-0.5)
	return float32(float64(c.w)/2 + dx*d), float32(float64(c.h)/2 + dy*d)
}
