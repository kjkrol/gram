package mapfile

import (
	"fmt"
	"image"
	"image/png"
	"io/fs"

	"github.com/kjkrol/gram/internal/f32"
)

// Heights is a ground's heights a corner a pixel: Columns by Rows of them, Pixel metres
// apart, the first row the north edge.
type Heights struct {
	Columns int
	Rows    int
	Pixel   float32
	// Lowest is the height of the value 0, Step the metres a step of the value.
	Lowest float32
	Step   float32
	values []uint16
}

func readHeights(fsys fs.FS, file string, pixel, lowest, step float32) (*Heights, error) {
	f, err := fsys.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("board: %s: %w", file, err)
	}
	grey, ok := img.(*image.Gray16)
	if !ok {
		return nil, fmt.Errorf("board: %s: not a 16-bit grey image", file)
	}
	if pixel <= 0 || step <= 0 {
		return nil, fmt.Errorf("board: %s: its pixel and step are not set", file)
	}
	w, h := grey.Rect.Dx(), grey.Rect.Dy()
	if w < 2 || h < 2 {
		return nil, fmt.Errorf("board: %s: %dx%d pixels are no ground", file, w, h)
	}
	hs := &Heights{Columns: w, Rows: h, Pixel: pixel, Lowest: lowest, Step: step,
		values: make([]uint16, w*h)}
	for j := range h {
		for i := range w {
			o := j*grey.Stride + i*2
			hs.values[j*w+i] = uint16(grey.Pix[o])<<8 | uint16(grey.Pix[o+1])
		}
	}
	return hs, nil
}

func (h *Heights) covers(width, height float32) bool {
	return float32(h.Columns-1)*h.Pixel >= width && float32(h.Rows-1)*h.Pixel >= height
}

// Value is the value kept at a pixel.
func (h *Heights) Value(i, j int) uint16 { return h.values[j*h.Columns+i] }

// onPixel is how near a pixel, in pixels, a place is on it
const onPixel = 1e-4

// At is the height at a place of the world: a pixel's at a pixel, even between them, the
// edge's past it.
func (h *Heights) At(x, y float32) float32 {
	x = min(max(x/h.Pixel, 0), float32(h.Columns-1))
	y = min(max(y/h.Pixel, 0), float32(h.Rows-1))
	// on a pixel the height is the pixel's own, whatever the division left of the place
	if r := f32.Round(x); f32.Abs(x-r) < onPixel {
		x = r
	}
	if r := f32.Round(y); f32.Abs(y-r) < onPixel {
		y = r
	}
	i, j := min(int(x), h.Columns-2), min(int(y), h.Rows-2)
	tx, ty := x-float32(i), y-float32(j)
	at := func(i, j int) float32 { return h.Lowest + float32(h.values[j*h.Columns+i])*h.Step }
	a := at(i, j)*(1-tx) + at(i+1, j)*tx
	b := at(i, j+1)*(1-tx) + at(i+1, j+1)*tx
	return a*(1-ty) + b*ty
}

// cut is the heights from the pixel (i0, j0) to (i1, j1), every one in every-th of them
func (h *Heights) cut(i0, j0, i1, j1, every int) *Heights {
	out := &Heights{
		Columns: (i1-i0)/every + 1,
		Rows:    (j1-j0)/every + 1,
		Pixel:   h.Pixel * float32(every),
		Lowest:  h.Lowest,
		Step:    h.Step,
	}
	out.values = make([]uint16, out.Columns*out.Rows)
	for j := range out.Rows {
		for i := range out.Columns {
			out.values[j*out.Columns+i] = h.Value(i0+i*every, j0+j*every)
		}
	}
	return out
}

// land is whether some height is over zero
func (h *Heights) land() bool {
	for _, v := range h.values {
		if h.Lowest+float32(v)*h.Step > 0 {
			return true
		}
	}
	return false
}

func (h *Heights) image() *image.Gray16 {
	img := image.NewGray16(image.Rect(0, 0, h.Columns, h.Rows))
	for i, v := range h.values {
		img.Pix[2*i], img.Pix[2*i+1] = byte(v>>8), byte(v)
	}
	return img
}
