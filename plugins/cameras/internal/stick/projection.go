package stick

import (
	"math"

	"github.com/kjkrol/gram/camera"
)

var _ camera.Projection = projection{}

// The 2:1 view of Transport Tycoon: x runs down-right and y down-left on the screen. Heading and
// Pitch are radians, the world turning clockwise on the screen as Heading grows; flat is the same
// world from above, heights not drawn.
type projection struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
	Heading            float32
	Pitch              float32
	MinPitch           float32
	flat               bool
	// of Heading plus the eighth turn of the 2:1 view: the ground runs down the screen (sin, cos)
	sin, cos float32
	// screen units a cell runs down the screen, and a unit of height lifts a point, at Pitch
	down, lift float32
}

// 30° the flattest: flatter, the near relief hides what lies behind it.
const (
	defaultMinPitch = math.Pi / 6
	maxPitch        = math.Pi / 2
)

func (p projection) tilted(pitch float32) projection {
	flat := math.Asin(math.Min(float64(p.TileH/p.TileW), 1)) // the 2:1 view's pitch
	pt := math.Min(math.Max(float64(pitch), float64(p.MinPitch)), maxPitch)
	p.Pitch = float32(pt)
	p.down = p.TileW / math.Sqrt2 * float32(math.Sin(pt))
	p.lift = p.HeightUnit * float32(math.Cos(pt)/math.Cos(flat))
	return p
}

func (p projection) turned(heading float32) projection {
	h := math.Mod(float64(heading), 2*math.Pi)
	if h < 0 {
		h += 2 * math.Pi
	}
	p.Heading = float32(h)
	// float32 of both, so at an eighth turn they are equal and a row's cells tie in depth
	s, c := math.Sincos(h + math.Pi/4)
	p.sin, p.cos = float32(s), float32(c)
	return p
}

func (p projection) withDefaults() projection {
	if p.Cell <= 0 {
		panic("isometry: the view needs the world size of a cell")
	}
	if p.TileW == 0 {
		p.TileW = 64
	}
	if p.TileH == 0 {
		p.TileH = 32
	}
	if p.HeightUnit == 0 {
		p.HeightUnit = 1
	}
	if p.Headroom == 0 {
		p.Headroom = defaultHeadroom
	}
	if p.MinPitch <= 0 {
		p.MinPitch = defaultMinPitch
	}
	p.MinPitch = float32(math.Min(float64(p.MinPitch), maxPitch))
	if p.Pitch == 0 {
		p.Pitch = float32(math.Asin(math.Min(float64(p.TileH/p.TileW), 1)))
	}
	return p.turned(p.Heading).tilted(p.Pitch)
}

func (p projection) viewed(flat bool) projection {
	p.flat = flat
	return p
}

func (p projection) Project(x, y, z float32) (float32, float32) {
	if p.flat {
		return x, y
	}
	u, v := x/p.Cell, y/p.Cell
	across, down := u*p.cos-v*p.sin, u*p.sin+v*p.cos
	return across * p.TileW / math.Sqrt2, down*p.down - z*p.lift
}

func (p projection) Unproject(sx, sy, z float32) (float32, float32) {
	if p.flat {
		return sx, sy
	}
	across, down := sx/(p.TileW/math.Sqrt2), (sy+z*p.lift)/p.down
	return (across*p.cos + down*p.sin) * p.Cell, (down*p.cos - across*p.sin) * p.Cell
}

// The middle of the cell: everything in one cell ties with its tile, the depth buffer saying what
// covers what.
func (p projection) Depth(x, y, _ float32) float32 {
	if p.flat {
		return y
	}
	u := float32(math.Floor(float64(x/p.Cell))) + 0.5
	v := float32(math.Floor(float64(y/p.Cell))) + 0.5
	return u*p.sin + v*p.cos
}

func (projection) Wraps() bool   { return false }
func (p projection) Sorts() bool { return !p.flat }

func (p projection) Toward() [3]float32 {
	if p.flat {
		return [3]float32{0, 0, 1}
	}
	across := p.Cell * p.lift
	n := float32(math.Sqrt(float64(across*across + p.down*p.down)))
	return [3]float32{p.sin * across / n, p.cos * across / n, p.down / n}
}
