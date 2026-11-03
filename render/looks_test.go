package render_test

import (
	stdimage "image"
	"image/color"
	"math"
	"testing"

	"github.com/kjkrol/gram/render"
)

// blobOf is what Blob makes of draw in a sprite of size
func blobOf(draw render.SpriteDrawer, size int) *render.Canvas {
	dst := &render.Canvas{RGBA: stdimage.NewRGBA(stdimage.Rect(0, 0, size, size))}
	render.Blob(draw)(dst, size)
	return dst
}

// alpha is the sum of a canvas's alpha, in whole pixels
func alpha(c *render.Canvas) float64 {
	var a float64
	for i := 3; i < len(c.Pix); i += 4 {
		a += float64(c.Pix[i])
	}
	return a / 255
}

func TestBlob_AHalfClearSquareIsADiscOfItsColourAsBigAsItCovers(t *testing.T) {
	const size = 64
	blob := blobOf(render.Solid(color.RGBA{R: 255, A: 128}), size)
	radius := size / 2 * math.Sqrt(128.0/255)
	red := color.RGBA{R: 255, A: 255}
	if got := blob.RGBAAt(size/2, size/2); got != red {
		t.Errorf("the middle is %v, want red at full alpha", got)
	}
	if got := blob.RGBAAt(size/2+int(radius)-2, size/2); got != red {
		t.Errorf("two pixels inside the radius %.2f: %v, want red", radius, got)
	}
	outside := []stdimage.Point{{size/2 + int(radius) + 2, size / 2}, {0, 0}, {size - 1, 0}}
	for _, p := range outside {
		if got := blob.RGBAAt(p.X, p.Y); got.A != 0 {
			t.Errorf("at %v, outside the radius %.2f: %v, want clear", p, radius, got)
		}
	}
	if got, want := alpha(blob), math.Pi*radius*radius; math.Abs(got-want) > want/100 {
		t.Errorf("the disc covers %.1f pixels, want %.1f", got, want)
	}
}

func TestBlob_TheColourIsTheMeanWeightedByAlpha(t *testing.T) {
	const size = 64
	halves := func(dst *render.Canvas, size int) {
		half := float32(size) / 2
		dst.FillRect(0, 0, float32(size), half, color.RGBA{R: 255, A: 255})
		dst.FillRect(0, half, float32(size), half, color.RGBA{B: 255, A: 128})
	}
	blob := blobOf(halves, size)
	// premultiplied, the red half sums 255 of red and of alpha a pixel, the blue 128 of each
	r, b := math.Round(255*255.0/383), math.Round(128*255.0/383)
	want := color.RGBA{R: uint8(r), B: uint8(b), A: 255}
	if got := blob.RGBAAt(size/2, size/2); got != want {
		t.Errorf("the middle is %v, want %v", got, want)
	}
	radius := size / 2 * math.Sqrt(383.0/510)
	if got, want := alpha(blob), math.Pi*radius*radius; math.Abs(got-want) > want/100 {
		t.Errorf("the disc covers %.1f pixels, want %.1f", got, want)
	}
}

func TestBlob_OfNothingIsNothing(t *testing.T) {
	if got := alpha(blobOf(func(*render.Canvas, int) {}, 16)); got != 0 {
		t.Errorf("a blob of a clear sprite covers %v pixels, want none", got)
	}
}
