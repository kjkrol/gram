package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/kjkrol/gram/render/gpu"
)

// Font is a typeface at a size, its glyphs drawn on demand into a sheet of its own: text in any
// script the face has — Polish letters with Go's own fonts.
type Font struct {
	face    font.Face
	ascent  float32
	height  float32
	glyphs  map[rune]glyph
	sheet   *Image
	pix     *image.RGBA // the sheet as drawn on the CPU, white with the coverage as alpha
	x, y    int         // where the next glyph goes in the sheet
	row     int         // the tallest glyph of the row being filled
	written bool        // pix is on the GPU as it stands
}

// glyph is one rune's place in the sheet and how it lies against the pen.
type glyph struct {
	src     image.Rectangle // in the sheet
	dx, dy  float32         // its top-left from the pen on the baseline
	advance float32
	ok      bool // the face has it
}

const sheetSide = 1024

// NewFont is the face of the TrueType or OpenType font ttf at size pixels.
func NewFont(ttf []byte, size float64) (*Font, error) {
	f, err := opentype.Parse(ttf)
	if err != nil {
		return nil, fmt.Errorf("render: font: %w", err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("render: font: %w", err)
	}
	m := face.Metrics()
	return &Font{
		face:   face,
		ascent: float32(m.Ascent.Ceil()),
		height: float32(m.Height.Ceil()),
		glyphs: map[rune]glyph{},
		pix:    image.NewRGBA(image.Rect(0, 0, sheetSide, sheetSide)),
	}, nil
}

var defaultFont = sync.OnceValue(func() *Font {
	f, err := NewFont(goregular.TTF, 14)
	if err != nil {
		panic(err)
	}
	return f
})

// DefaultFont is Go Regular at 14 pixels, which has the Latin letters of every European language.
func DefaultFont() *Font { return defaultFont() }

// LineHeight is how far one line of text lies under the one before it.
func (f *Font) LineHeight() float32 { return f.height }

// Measure is how wide the longest line of s is and how high its lines are.
func (f *Font) Measure(s string) (w, h float32) {
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		w = max(w, f.width(line))
	}
	return w, f.height * float32(len(lines))
}

func (f *Font) width(line string) float32 {
	var w float32
	prev := rune(-1)
	for _, r := range line {
		if prev >= 0 {
			w += fixedToFloat(f.face.Kern(prev, r))
		}
		w += f.glyph(r).advance
		prev = r
	}
	return w
}

// glyph is r's glyph, drawn into the sheet at its first use.
func (f *Font) glyph(r rune) glyph {
	if g, ok := f.glyphs[r]; ok {
		return g
	}
	dr, mask, mp, advance, ok := f.face.Glyph(fixed.Point26_6{}, r)
	g := glyph{advance: fixedToFloat(advance), ok: ok}
	if ok && !dr.Empty() {
		w, h := dr.Dx(), dr.Dy()
		if f.x+w+1 > sheetSide {
			f.x, f.y, f.row = 0, f.y+f.row+1, 0
		}
		if f.y+h+1 <= sheetSide {
			at := image.Rect(f.x, f.y, f.x+w, f.y+h)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					_, _, _, a := mask.At(mp.X+x, mp.Y+y).RGBA()
					c := uint8(a >> 8)
					f.pix.SetRGBA(at.Min.X+x, at.Min.Y+y, color.RGBA{c, c, c, c})
				}
			}
			g.src, g.dx, g.dy = at, float32(dr.Min.X), float32(dr.Min.Y)
			f.x, f.row = f.x+w+1, max(f.row, h)
			f.written = false
		}
	}
	f.glyphs[r] = g
	return g
}

// Has reports whether the face has a glyph for r.
func (f *Font) Has(r rune) bool { return f.glyph(r).ok }

// DrawText draws s in f, c its colour, its first line's top-left at (x, y) of dst, a line under
// another at each newline.
func DrawText(dst *Image, f *Font, s string, x, y float32, c color.Color) {
	var pieces []gpu.Piece
	pen := y + f.ascent
	for _, line := range strings.Split(s, "\n") {
		px := x
		prev := rune(-1)
		for _, r := range line {
			if prev >= 0 {
				px += fixedToFloat(f.face.Kern(prev, r))
			}
			g := f.glyph(r)
			if !g.src.Empty() {
				pieces = append(pieces, gpu.Piece{
					DstX: float32(math.Round(float64(px + g.dx))), DstY: float32(math.Round(float64(pen + g.dy))),
					SrcX: float32(g.src.Min.X), SrcY: float32(g.src.Min.Y), W: float32(g.src.Dx()), H: float32(g.src.Dy())})
			}
			px += g.advance
			prev = r
		}
		pen += f.height
	}
	if len(pieces) == 0 {
		return
	}
	if f.sheet == nil {
		f.sheet = NewImage(sheetSide, sheetSide)
	}
	if !f.written {
		f.sheet.WritePixels(f.pix.Pix)
		f.written = true
	}
	cr, cg, cb, ca := rgbaOf(c)
	gpu.DrawPieces(dst.gpu(), f.sheet.gpu(), pieces, [4]float32{cr, cg, cb, ca}, gpu.SourceOver)
}

func fixedToFloat(v fixed.Int26_6) float32 { return float32(v) / 64 }
