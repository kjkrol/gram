package isometry

import (
	"math"

	"github.com/kjkrol/gram/camera"
)

var _ camera.Projection = projection{}

// projection is the 2:1 view of Transport Tycoon: a Cell-sized square of the world is a TileW x
// TileH diamond, the world's x axis runs down-right and its y axis down-left, and a height lifts a
// point HeightUnit screen units per world unit. Headroom is how far above the ground Visible looks
// for sprites (default 64). Zero TileW, TileH and HeightUnit are 64, 32 and 1. Heading turns the
// view round the vertical, the world clockwise on the screen, radians from the 2:1 view; Pitch is
// how steeply the eye looks down on the ground, from minPitch to straight down, the 2:1 view's
// asin(TileH/TileW) when zero.
type projection struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
	Heading            float32
	Pitch              float32
	// sin and cos of Heading and the eighth turn of the 2:1 view: the way the ground runs down the
	// screen is (sin, cos) in cells
	sin, cos float32
	// screen units a cell runs down the screen, and a unit of height lifts a point, at Pitch
	down, lift float32
}

// The steepest and the flattest the eye may look down on the ground.
const (
	minPitch = math.Pi / 18 // ten degrees
	maxPitch = math.Pi / 2
)

// tilted is p looking down at pitch, held between minPitch and maxPitch: the ground runs down the
// screen as its sine, a height lifts a point as its cosine, both as the 2:1 view has them there.
func (p projection) tilted(pitch float32) projection {
	flat := math.Asin(math.Min(float64(p.TileH/p.TileW), 1)) // the 2:1 view's pitch
	pt := math.Min(math.Max(float64(pitch), minPitch), maxPitch)
	p.Pitch = float32(pt)
	p.down = p.TileW / math.Sqrt2 * float32(math.Sin(pt))
	p.lift = p.HeightUnit * float32(math.Cos(pt)/math.Cos(flat))
	return p
}

// turned is p looking from heading, 0 up to 2π.
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

// withDefaults fills the zero fields; Cell must be set.
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
		p.Headroom = 64
	}
	if p.Pitch == 0 {
		p.Pitch = float32(math.Asin(math.Min(float64(p.TileH/p.TileW), 1)))
	}
	return p.turned(p.Heading).tilted(p.Pitch)
}

func (p projection) Project(x, y, z float32) (float32, float32) {
	u, v := x/p.Cell, y/p.Cell
	across, down := u*p.cos-v*p.sin, u*p.sin+v*p.cos
	return across * p.TileW / math.Sqrt2, down*p.down - z*p.lift
}

func (p projection) Unproject(sx, sy, z float32) (float32, float32) {
	across, down := sx/(p.TileW/math.Sqrt2), (sy+z*p.lift)/p.down
	return (across*p.cos + down*p.sin) * p.Cell, (down*p.cos - across*p.sin) * p.Cell
}

// Depth is how far down the screen the middle of the cell under the point lies: rows further back
// are smaller, and everything in one cell ties with its tile, so what a Composer is handed on a
// higher tier — the entities standing on it — is drawn over it and under what lies in front.
func (p projection) Depth(x, y, _ float32) float32 {
	u := float32(math.Floor(float64(x/p.Cell))) + 0.5
	v := float32(math.Floor(float64(y/p.Cell))) + 0.5
	return u*p.sin + v*p.cos
}

func (projection) Wraps() bool { return false }
func (projection) Sorts() bool { return true }

// Toward is the way along which every point projects to one spot of the screen: along the ground
// down the screen — the diagonal towards +x and +y unturned — rising as far as a step that way runs
// down the screen for each unit a height lifts a point; straight up looking straight down.
func (p projection) Toward() [3]float32 {
	across := p.Cell * p.lift
	n := float32(math.Sqrt(float64(across*across + p.down*p.down)))
	return [3]float32{p.sin * across / n, p.cos * across / n, p.down / n}
}
