package render_test

import (
	stdimage "image"
	"image/color"
	"testing"

	"github.com/kjkrol/gram/render"
)

func TestPhases_FrameRunsFromTheFirstToTheLastAlongTheCourse(t *testing.T) {
	p := render.Phased(4, func(int) render.SpriteDrawer { return render.Solid(color.RGBA{}) })
	for progress, want := range map[float32]int{0: 0, 0.24: 0, 0.26: 1, 0.5: 2, 0.99: 3, 1: 3, 2: 3} {
		if got := p.Frame(progress); got != want {
			t.Errorf("at %v: frame %d, want %d", progress, got, want)
		}
	}
}

func TestFromImage_ScalesPixelForPixelWithoutSmoothing(t *testing.T) {
	src := stdimage.NewRGBA(stdimage.Rect(0, 0, 2, 2))
	red := color.RGBA{R: 255, A: 255}
	src.SetRGBA(0, 0, red)
	dst := &render.Canvas{RGBA: stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 4))}
	render.FromImage(src)(dst, 4)
	if got := dst.RGBAAt(1, 1); got != red {
		t.Errorf("the red pixel scaled twice over: (1,1) is %v, want red", got)
	}
	if got := dst.RGBAAt(2, 1); got.A != 0 {
		t.Errorf("past the red pixel: (2,1) is %v, want clear, no smoothing", got)
	}
}
