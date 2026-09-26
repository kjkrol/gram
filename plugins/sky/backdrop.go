package sky

import (
	"image/color"
	"math"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var _ render.Source = (*backdrop)(nil)

// backdrop is the sky behind the world: the viewport filled in the colour of the world's sky,
// behind everything, whenever the ground does not cover all of it — beyond the world's edge, above
// a low view; a view the ground covers draws none.
type backdrop struct{ world *world.Plugin }

func (*backdrop) Init(*goke.SysInit) {}

func (b *backdrop) Compose(f *render.Frame, cam camera.Camera) {
	w, h := cam.Viewport()
	if b.covered(cam, w, h) {
		return
	}
	sky := b.world.Sun().Daylight().Sky
	c := color.RGBA{A: 255}
	c.R, c.G, c.B = channel(sky[0]), channel(sky[1]), channel(sky[2])
	f.Soft(render.Backdrop, float32(math.Inf(-1)), render.Corners{{0, 0}, {w, 0}, {0, h}, {w, h}}, c, render.Fade{})
}

// covered reports whether the ground covers the w x h viewport: a wrapping world seen through a
// projection that wraps, or every corner of the screen over the world.
func (b *backdrop) covered(cam camera.Camera, w, h float32) bool {
	space := b.world.Res.Config.Space
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

// channel is v, 0 to 1, as a colour channel.
func channel(v float32) uint8 { return uint8(min(max(v, 0), 1)*255 + 0.5) }
