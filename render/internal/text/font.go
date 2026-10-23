package text

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/kjkrol/gram/internal/gpu"
	"github.com/kjkrol/gram/render/internal/gfx"
)

// Font is a typeface at a size, its glyphs drawn on demand into a sheet of its own.
type Font struct {
	face    font.Face
	ascent  float32
	height  float32
	glyphs  map[rune]glyph
	sheet   *gfx.Image
	pix     *image.RGBA // white with the coverage as alpha
	x, y    int
	row     int
	written bool
	pieces  []gpu.Piece // Draw's, reused: a font is drawn from one goroutine, the frame's
}

type glyph struct {
	src     image.Rectangle
	dx, dy  float32 // its top-left from the pen on the baseline
	advance float32
	ok      bool
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

var defaultBoldFont = sync.OnceValue(func() *Font {
	f, err := NewFont(gobold.TTF, 14)
	if err != nil {
		panic(err)
	}
	return f
})

// DefaultBoldFont is Go Bold at 14 pixels, DefaultFont's weight for a heading.
func DefaultBoldFont() *Font { return defaultBoldFont() }

var defaultLargeFont = sync.OnceValue(func() *Font {
	f, err := NewFont(gobold.TTF, 20)
	if err != nil {
		panic(err)
	}
	return f
})

// DefaultLargeFont is Go Bold at 20 pixels, for a screen's title.
func DefaultLargeFont() *Font { return defaultLargeFont() }

// LineHeight is how far one line of text lies under the one before it.
func (f *Font) LineHeight() float32 { return f.height }

// Measure is how wide the longest line of s is and how high its lines are.
func (f *Font) Measure(s string) (w, h float32) {
	for more := true; more; {
		var line string
		line, s, more = nextLine(s)
		w, h = max(w, f.width(line)), h+f.height
	}
	return w, h
}

// nextLine cuts the first line off s at its newline; without one the line is all of s
func nextLine(s string) (line, rest string, more bool) {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i], s[i+1:], true
	}
	return s, "", false
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

// Draw draws s in the font, c its colour, its first line's top-left at (x, y) of dst, a line
// under another at each newline.
func (f *Font) Draw(dst *gfx.Image, s string, x, y float32, c color.Color) {
	f.pieces = f.pieces[:0]
	pen := y + f.ascent
	for more := true; more; {
		var line string
		line, s, more = nextLine(s)
		px := x
		prev := rune(-1)
		for _, r := range line {
			if prev >= 0 {
				px += fixedToFloat(f.face.Kern(prev, r))
			}
			g := f.glyph(r)
			if !g.src.Empty() {
				f.pieces = append(f.pieces, gpu.Piece{
					DstX: float32(math.Round(float64(px + g.dx))), DstY: float32(math.Round(float64(pen + g.dy))),
					SrcX: float32(g.src.Min.X), SrcY: float32(g.src.Min.Y), W: float32(g.src.Dx()), H: float32(g.src.Dy())})
			}
			px += g.advance
			prev = r
		}
		pen += f.height
	}
	if len(f.pieces) == 0 {
		return
	}
	if f.sheet == nil {
		f.sheet = gfx.NewImage(sheetSide, sheetSide)
	}
	if !f.written {
		f.sheet.WritePixels(f.pix.Pix)
		f.written = true
	}
	cr, cg, cb, ca := gfx.Premultiply(c)
	gpu.DrawPieces(gfx.GPU(dst), gfx.GPU(f.sheet), f.pieces, [4]float32{cr, cg, cb, ca}, gpu.SourceOver)
}

func fixedToFloat(v fixed.Int26_6) float32 { return float32(v) / 64 }
