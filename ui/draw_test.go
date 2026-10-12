package ui

import (
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// needGPU readies a device without a window; a machine without one skips.
func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

// still is a surface of one colour.
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
	needGPU(t)
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
