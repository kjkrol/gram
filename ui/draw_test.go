package ui

import (
	"image/color"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

type still struct{ img *render.Image }

func (s *still) Resize(w, h int) {
	s.img = render.NewImage(w, h)
	s.img.Fill(color.RGBA{R: 255, A: 255})
}
func (s *still) Draw() *render.Image { return s.img }

func pixel(img *render.Image, x, y int) [4]byte {
	b := img.Bounds()
	pix := make([]byte, 4*b.Dx()*b.Dy())
	img.ReadPixels(pix)
	k := 4 * (y*b.Dx() + x)
	return [4]byte{pix[k], pix[k+1], pix[k+2], pix[k+3]}
}

func TestImage_MaskedByACircleLeavesTheCornersOfItsBoxBare(t *testing.T) {
	gputest.Need(t)
	dst := render.NewImage(64, 64)
	dst.Clear()
	e := Image(&still{}).Masked(Circle)
	e.lay(box(0, 0, 64, 64))
	e.paint(dst)
	if p := pixel(dst, 32, 32); p[0] != 255 {
		t.Errorf("the middle is %v, want the picture's red", p)
	}
	if p := pixel(dst, 1, 1); p[3] != 0 {
		t.Errorf("the corner is %v, want bare", p)
	}
}

type filling struct{}

func (filling) DrawWorld(screen *render.Image, _ camera.Camera) {
	screen.Fill(color.RGBA{R: 255, A: 255})
}

func TestImage_AFeedAtAResolutionFillsItsBox(t *testing.T) {
	gputest.Need(t)
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.(camera.Resolved).SetResolution(0.5)
	feed := render.NewFeed(cam, filling{})
	dst := render.NewImage(64, 64)
	for name, c := range map[string]struct {
		box      [4]float32
		red, off [2]int
		masked   bool
	}{
		"in a box":   {box: [4]float32{0, 0, 40, 40}, red: [2]int{39, 39}, off: [2]int{41, 41}},
		"the screen": {box: [4]float32{0, 0, 64, 64}, red: [2]int{63, 63}, off: [2]int{-1, -1}},
		"in a circle": {
			box:    [4]float32{0, 0, 64, 64},
			red:    [2]int{32, 32},
			off:    [2]int{1, 1},
			masked: true,
		},
	} {
		dst.Clear()
		e := Image(feed)
		if c.masked {
			e.Masked(Circle)
		}
		e.lay(box(c.box[0], c.box[1], c.box[2], c.box[3]))
		e.paint(dst)
		if w, h := cam.Viewport(); w != float32(c.box[2])/2 || h != float32(c.box[3])/2 {
			t.Errorf("%s: the camera's viewport is %v x %v, want half the box", name, w, h)
		}
		if p := pixel(dst, c.red[0], c.red[1]); p[0] != 255 || p[3] != 255 {
			t.Errorf("%s: pixel %v is %v, want the picture's red", name, c.red, p)
		}
		if c.off[0] < 0 {
			continue
		}
		if p := pixel(dst, c.off[0], c.off[1]); p[3] != 0 {
			t.Errorf("%s: pixel %v is %v, want bare", name, c.off, p)
		}
	}
}
