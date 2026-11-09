package text

import (
	"strconv"
	"sync"

	"golang.org/x/image/font/gofont/gobold"

	"github.com/kjkrol/gram/render/internal/gpu"
)

// pixels: the size of the face a badge's number is written in
const numberSize = 10

var numberFont = sync.OnceValue(func() *Font {
	f, err := NewFont(gobold.TTF, numberSize)
	if err != nil {
		panic(err)
	}
	return f
})

// NumberFont is Go Bold at 10 pixels, the face a badge's number is drawn in.
func NumberFont() *Font { return numberFont() }

// NumberGlyphs appends to dst the glyphs of n written in f, as Glyphs lays a string.
func NumberGlyphs(f *Font, n int, dst []gpu.Piece) []gpu.Piece {
	var digits [20]byte
	return f.lay(dst, string(strconv.AppendInt(digits[:0], int64(n), 10)), 0, 0, false)
}

// measuring is a copy of NumberFont's face of its own, held while it measures a number
var measuring struct {
	sync.Mutex
	font   *Font
	pieces []gpu.Piece
}

// NumberInk is the box round the ink of n written in NumberFont, as NumberGlyphs lays it: its
// top-left and its size. It may be asked on any goroutine.
func NumberInk(n int) (x, y, w, h float32) {
	m := &measuring
	m.Lock()
	defer m.Unlock()
	if m.font == nil {
		f, err := NewFont(gobold.TTF, numberSize)
		if err != nil {
			panic(err)
		}
		m.font = f
	}
	m.pieces = NumberGlyphs(m.font, n, m.pieces[:0])
	return Ink(m.pieces)
}

// Ink is the box round the glyphs laid: its top-left and its size; zero for none.
func Ink(glyphs []gpu.Piece) (x, y, w, h float32) {
	if len(glyphs) == 0 {
		return 0, 0, 0, 0
	}
	x0, y0 := glyphs[0].DstX, glyphs[0].DstY
	x1, y1 := x0+glyphs[0].W, y0+glyphs[0].H
	for _, p := range glyphs[1:] {
		x0, y0 = min(x0, p.DstX), min(y0, p.DstY)
		x1, y1 = max(x1, p.DstX+p.W), max(y1, p.DstY+p.H)
	}
	return x0, y0, x1 - x0, y1 - y0
}
