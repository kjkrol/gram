package render

import (
	"github.com/kjkrol/gram/render/internal/instanced"
	"github.com/kjkrol/gram/render/internal/text"
)

// Font is a typeface at a size, its glyphs drawn on demand into a sheet of its own; Draw draws a
// string in it.
type Font = text.Font

// Labels is text a Direct source draws on the GPU by points of the world, standing upright on
// each point for the depth test, and badges with a number in.
type Labels = instanced.Labels

// BadgeShape is the outline of a badge Labels draws.
type BadgeShape = instanced.BadgeShape

const (
	// BadgeDisc is a round badge.
	BadgeDisc = instanced.BadgeDisc
	// BadgeDiamond is a square badge standing on a corner.
	BadgeDiamond = instanced.BadgeDiamond
)

// BadgeWidth is how many shown pixels across a badge of shape at least size across is to hold n
// written in its middle, nothing written where n is not positive; it may be asked on any goroutine.
func BadgeWidth(n int, shape BadgeShape, size float32) float32 {
	return instanced.BadgeWidth(n, shape, size)
}

// NewFont is the face of the TrueType or OpenType font ttf at size pixels.
func NewFont(ttf []byte, size float64) (*Font, error) { return text.NewFont(ttf, size) }

// DefaultFont is Go Regular at 14 pixels, which has the Latin letters of every European language.
func DefaultFont() *Font { return text.DefaultFont() }

// DefaultBoldFont is Go Bold at 14 pixels, DefaultFont's weight for a heading.
func DefaultBoldFont() *Font { return text.DefaultBoldFont() }

// DefaultLargeFont is Go Bold at 28 pixels, for a screen's title.
func DefaultLargeFont() *Font { return text.DefaultLargeFont() }

// DebugPrintAt prints s with its top-left corner at (x, y), a line of it under another at each
// newline, in the fixed debug text.
func DebugPrintAt(dst *Image, s string, x, y int) { text.DebugPrintAt(dst, s, x, y) }
