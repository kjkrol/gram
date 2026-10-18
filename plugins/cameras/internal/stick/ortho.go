package stick

import (
	"math"

	"github.com/kjkrol/gram/camera"
)

// What every lens of a map in relief shares: a Cell-sized square drawn isometrically as a
// TileW x TileH diamond, a height lifting a point HeightUnit screen units a world unit there,
// Headroom how far over the highest ground sprites are looked for, MinPitch (radians) the
// flattest the stick leans.
type optics struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
	MinPitch           float32
}

// 30° the flattest: flatter, the near relief hides what lies behind it.
const (
	defaultMinPitch = math.Pi / 6
	maxPitch        = math.Pi / 2
)

func (o optics) withDefaults() optics {
	if o.Cell <= 0 {
		panic("cameras: a view in relief needs the world size of a cell")
	}
	if o.TileW == 0 {
		o.TileW = 64
	}
	if o.TileH == 0 {
		o.TileH = 32
	}
	if o.HeightUnit == 0 {
		o.HeightUnit = 1
	}
	if o.Headroom == 0 {
		o.Headroom = defaultHeadroom
	}
	if o.MinPitch <= 0 {
		o.MinPitch = defaultMinPitch
	}
	o.MinPitch = min(o.MinPitch, maxPitch)
	return o
}

// the 2:1 view's pitch
func (o optics) isoPitch() float64 { return math.Asin(math.Min(float64(o.TileH/o.TileW), 1)) }

// screen units a world unit spans across the screen in the 2:1 view at its own zoom
func (o optics) tile() float32 { return o.TileW / (math.Sqrt2 * o.Cell) }

// screen units a unit of height lifts a point at a true pixel a world unit, looking as the 2:1
// view does
func (o optics) rise() float32 {
	return o.HeightUnit / (o.tile() * float32(math.Cos(o.isoPitch())))
}

var _ camera.Projection = ortho{}

// ortho is the stick's parallel view at a true screen unit a world unit: the ground turned by
// turn (x running right at 0, the world turning clockwise as it grows) and tilted by pitch, a
// height lifting a point the more the flatter the stick.
type ortho struct {
	cell float32
	// of the turn; of the pitch, exactly 1 and 0 straight down
	sin, cos float32
	sp, cp   float32
	// screen units a unit of height lifts a point
	lift float32
}

func newOrtho(o optics, turn, pitch float64) ortho {
	// float32 of both, so at an eighth turn they are equal and a row's cells tie in depth
	s, c := math.Sincos(turn)
	p := ortho{cell: o.Cell, sin: float32(s), cos: float32(c), sp: 1}
	if pitch < maxPitch {
		sp, cp := math.Sincos(pitch)
		p.sp, p.cp = float32(sp), float32(cp)
	}
	p.lift = o.rise() * p.cp
	return p
}

func (p ortho) Project(x, y, z float32) (float32, float32) {
	return x*p.cos - y*p.sin, (x*p.sin+y*p.cos)*p.sp - z*p.lift
}

func (p ortho) Unproject(sx, sy, z float32) (float32, float32) {
	down := (sy + z*p.lift) / p.sp
	return sx*p.cos + down*p.sin, down*p.cos - sx*p.sin
}

// Straight down the way the screen runs down; leaning, the middle of the cell, so everything in
// one cell ties with its tile, the depth buffer saying what covers what.
func (p ortho) Depth(x, y, _ float32) float32 {
	if !p.Sorts() {
		return x*p.sin + y*p.cos
	}
	u := float32(math.Floor(float64(x/p.cell))) + 0.5
	v := float32(math.Floor(float64(y/p.cell))) + 0.5
	return u*p.sin + v*p.cos
}

func (ortho) Wraps() bool   { return false }
func (p ortho) Sorts() bool { return p.cp != 0 }

func (p ortho) Toward() [3]float32 {
	n := float32(math.Hypot(float64(p.lift), float64(p.sp)))
	return [3]float32{p.sin * p.lift / n, p.cos * p.lift / n, p.sp / n}
}

// the world step along the ground at height z that moves a point (dsx, dsy) on the screen, at
// scale screen units a world unit
func (p ortho) step(dsx, dsy, scale float64) (dx, dy float64) {
	s, c := float64(p.sin), float64(p.cos)
	across, down := dsx/scale, dsy/(scale*float64(p.sp))
	return across*c + down*s, down*c - across*s
}
