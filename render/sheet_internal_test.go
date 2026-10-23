package render

import (
	stdimage "image"
	"image/color"
	"testing"
)

func TestSheet_AtCutsTheFrameOfTheColumnAndRow(t *testing.T) {
	src := stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 4))
	red := color.RGBA{R: 255, A: 255}
	src.SetRGBA(2, 0, red) // the second frame's first pixel, the frames 2 wide and 4 tall
	sheet := newSheet(src, 2, 4)
	dst := &Canvas{RGBA: stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 4))}
	sheet.At(1, 0)(dst, 4) // stretched twice across into the square sprite
	if got := dst.RGBAAt(1, 0); got != red {
		t.Errorf("frame (1, 0) begins with %v, want red", got)
	}
	if got := dst.RGBAAt(2, 0); got.A != 0 {
		t.Errorf("frame (1, 0) at (2, 0) is %v, want clear", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("a frame off the sheet drew without a panic")
		}
	}()
	sheet.At(2, 0)
}

func TestFromImage_ScalesPixelForPixelWithoutSmoothing(t *testing.T) {
	src := stdimage.NewRGBA(stdimage.Rect(0, 0, 2, 2))
	red := color.RGBA{R: 255, A: 255}
	src.SetRGBA(0, 0, red)
	dst := &Canvas{RGBA: stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 4))}
	fromImage(src)(dst, 4)
	if got := dst.RGBAAt(1, 1); got != red {
		t.Errorf("the red pixel scaled twice over: (1,1) is %v, want red", got)
	}
	if got := dst.RGBAAt(2, 1); got.A != 0 {
		t.Errorf("past the red pixel: (2,1) is %v, want clear, no smoothing", got)
	}
}
