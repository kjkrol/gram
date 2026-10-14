package render

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
)

// Sheet is an image of sprites laid out in cells of one size, a frame cut out by its column and
// row, pixel for pixel: drawn into a square sprite, a frame of another shape is stretched, and
// an entity's box of the frame's shape draws it true.
type Sheet struct {
	img  image.Image
	w, h int
}

// NewSheet is img as a sheet of frames w by h pixels.
func NewSheet(img image.Image, w, h int) Sheet {
	if img == nil || w <= 0 || h <= 0 {
		panic(fmt.Sprintf("render: a sheet of %v in cells of %d by %d", img, w, h))
	}
	return Sheet{img: img, w: w, h: h}
}

// DecodeSheet is the PNG data as a sheet of frames w by h pixels; name says which file in a
// panic.
func DecodeSheet(name string, data []byte, w, h int) Sheet {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Sprintf("render: %s: %v", name, err))
	}
	return NewSheet(img, w, h)
}

// At is the drawer of the frame in column col and row row, counted from zero.
func (s Sheet) At(col, row int) SpriteDrawer {
	r := image.Rect(col*s.w, row*s.h, (col+1)*s.w, (row+1)*s.h)
	if !r.In(s.img.Bounds()) {
		panic(fmt.Sprintf("render: frame %d, %d lies off the sheet %v", col, row, s.img.Bounds()))
	}
	sub, ok := s.img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		panic(fmt.Sprintf("render: a sheet of %T cuts no frames", s.img))
	}
	return FromImage(sub.SubImage(r))
}
