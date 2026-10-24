package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"time"

	"github.com/kjkrol/ceremony/effect"
	xdraw "golang.org/x/image/draw"
)

// Phases is a look of n frames spread along an effect's course, from the first as the effect
// begins to the last as it runs out, once; a look an atlas takes Under an effect that Lasts.
// One of Draw and DrawFacing draws a frame, the latter for each way a facing sprite looks.
type Phases struct {
	N          int
	Draw       func(frame int) SpriteDrawer
	DrawFacing func(angleDeg float64, frame int) SpriteDrawer
}

// Phased is the look of n frames along an effect's course, each drawn by draw for its frame.
func Phased(n int, draw func(frame int) SpriteDrawer) Phases {
	if n <= 0 || draw == nil {
		panic(fmt.Sprintf("render: a look phased over %d frames", n))
	}
	return Phases{N: n, Draw: draw}
}

// PhasedFacing is Phased for a sprite that faces n ways: draw draws each frame at each angle.
func PhasedFacing(n int, draw func(angleDeg float64, frame int) SpriteDrawer) Phases {
	if n <= 0 || draw == nil {
		panic(fmt.Sprintf("render: a look phased over %d frames", n))
	}
	return Phases{N: n, DrawFacing: draw}
}

// Loops is a look of N frames shown round and round while an effect is on: in turn, each for
// Period of game time, or to a Beat. One of Draw and DrawFacing draws a frame.
type Loops struct {
	N          int
	Period     time.Duration
	Beat       []Beat
	Draw       func(frame int) SpriteDrawer
	DrawFacing func(angleDeg float64, frame int) SpriteDrawer
}

// Beat is one span of a timed look: which frame is shown, for how long.
type Beat struct {
	Frame int
	Span  time.Duration
}

// Timed is the look going round a beat, frame after frame each for its span, drawn by draw
// once a frame named: a blink of 150 ms every 3 s is two beats of two frames.
func Timed(beat []Beat, draw func(frame int) SpriteDrawer) Loops {
	return Loops{N: framesOf(beat), Beat: beat, Draw: draw}
}

// TimedFacing is Timed for a sprite that faces n ways: draw draws each frame at each angle.
func TimedFacing(beat []Beat, draw func(angleDeg float64, frame int) SpriteDrawer) Loops {
	return Loops{N: framesOf(beat), Beat: beat, DrawFacing: draw}
}

func framesOf(beat []Beat) int {
	if len(beat) == 0 {
		panic("render: a look timed to no beat")
	}
	n := 0
	for _, b := range beat {
		if b.Span <= 0 || b.Frame < 0 {
			panic(fmt.Sprintf("render: a look timed to a beat of frame %d for %v", b.Frame, b.Span))
		}
		n = max(n, b.Frame+1)
	}
	return n
}

// Frame is the frame shown at the game time now.
func (l Loops) Frame(now time.Duration) int {
	if len(l.Beat) == 0 {
		return int(now/l.Period) % l.N
	}
	var cycle time.Duration
	for _, b := range l.Beat {
		cycle += b.Span
	}
	t := now % cycle
	for _, b := range l.Beat {
		if t < b.Span {
			return b.Frame
		}
		t -= b.Span
	}
	return l.Beat[len(l.Beat)-1].Frame
}

// LoopedFacing is the look of n frames round and round, each shown period, of a sprite that
// faces every way: draw draws each frame at each angle.
func LoopedFacing(n int, period time.Duration,
	draw func(angleDeg float64, frame int) SpriteDrawer) Loops {
	if n <= 0 || period <= 0 || draw == nil {
		panic(fmt.Sprintf("render: a look looped over %d frames of %v", n, period))
	}
	return Loops{N: n, Period: period, DrawFacing: draw}
}

// Frame is the frame shown at progress, 0 as the effect begins to 1 as it runs out.
func (p Phases) Frame(progress float32) int {
	return min(max(int(progress*float32(p.N)), 0), p.N-1)
}

// fromImage is a SpriteDrawer laying img over the whole sprite, scaled pixel for pixel without
// smoothing: a frame cut out of a sheet keeps its pixels.
func fromImage(img image.Image) SpriteDrawer {
	return func(dst *Canvas, size int) {
		xdraw.NearestNeighbor.Scale(dst.RGBA, image.Rect(0, 0, size, size), img, img.Bounds(),
			xdraw.Over, nil)
	}
}

// Look is the ways a look is painted: drawn once into the sheet (a SpriteDrawer), worked out
// per pixel every frame (a MaterialID), frames along an effect's course (Phases) or frames
// round and round by the clock (Loops).
type Look interface {
	~func(dst *Canvas, size int) | MaterialID | Phases | Loops
}

// Sheet is an image of sprites laid out in cells of one size, a frame cut out by its column and
// row, pixel for pixel: drawn into a square sprite, a frame of another shape is stretched, and
// an entity's box of the frame's shape draws it true.
type Sheet struct {
	img  image.Image
	w, h int
}

// newSheet is img as a sheet of frames w by h pixels.
func newSheet(img image.Image, w, h int) Sheet {
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
	return newSheet(img, w, h)
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
	return fromImage(sub.SubImage(r))
}

// Solid is a SpriteDrawer filling the whole sprite with c.
func Solid(c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		dst.FillRect(0, 0, float32(size), float32(size), c)
	}
}

// Border is a SpriteDrawer outlining the sprite with c, transparent inside.
func Border(c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if x <= 1 || x >= size-2 || y <= 1 || y >= size-2 {
					dst.Set(x, y, c)
				}
			}
		}
	}
}

// Diamond is a SpriteDrawer filling a diamond with c, transparent outside.
func Diamond(c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		cx, cy := size/2, size/2
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if abs(x-cx)+abs(y-cy) <= cx-1 {
					dst.Set(x, y, c)
				}
			}
		}
	}
}

// Cross is a SpriteDrawer filling a cross with c, transparent outside.
func Cross(c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				inH := y >= size/4 && y < size*3/4
				inV := x >= size/4 && x < size*3/4
				if inH || inV {
					dst.Set(x, y, c)
				}
			}
		}
	}
}

// Hexagon is a SpriteDrawer filling a pointy-top hexagon with c, transparent at the corners.
// Drawn over a cell of width √3·r and height 2·r it is regular.
func Hexagon(c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		half := float64(size) / 2
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				du := math.Abs(float64(x)+0.5-half) / half
				dv := math.Abs(float64(y)+0.5-half) / half
				if du <= 1 && dv <= 1-du/2 {
					dst.Set(x, y, c)
				}
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func endpointAt(size int, angleDeg float64) (x, y float32) {
	cx, cy := float32(size)/2, float32(size)/2
	rad := angleDeg * math.Pi / 180
	dx, dy := float32(math.Cos(rad)), -float32(math.Sin(rad))
	half := float32(size) / 2
	scale := half / max(abs32(dx), abs32(dy))
	return cx + dx*scale, cy + dy*scale
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func Arrow(angleDeg float64, width float32, c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		x0, y0 := endpointAt(size, angleDeg)
		cx, cy := float32(size)/2, float32(size)/2
		dst.StrokeLine(x0, y0, cx, cy, width, c)
	}
}

func Dot(radius float32, c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		dst.FillCircle(float32(size)/2, float32(size)/2, radius, c)
	}
}

// Appearance is the sprite an entity is drawn from and how much it bends in the wind, 0 not at
// all, 1 as far as the wind blows it.
type Appearance struct {
	SpriteID SpriteID
	Sway     float32
	// Angle turns the look about the middle of the entity's box, in degrees, 0 east, against the
	// clock with the screen's y growing down. The box stays axis-aligned: content past the circle
	// inscribed in it swings outside the box as the sprite turns.
	Angle float32
}

// Knob: an effect may turn it (effect.Knob).
func (Appearance) Knob() {}

var _ effect.Knob = Appearance{}
