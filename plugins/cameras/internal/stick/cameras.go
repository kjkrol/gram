package stick

import (
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
)

// Config is the views of a map in relief: a Cell-sized square of the world drawn isometrically
// as a TileW x TileH diamond (64 x 32 when zero), a height lifting a point HeightUnit screen
// units a world unit there (1), Headroom how far above the ground a camera looks for sprites (64),
// MinPitch the flattest the stick leans in degrees (30), and whether Tab reaches a Perspective
// seeing FieldOfView degrees top to bottom (45). Cell must be set.
type Config struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
	MinPitch           float32
	Perspective        bool
	FieldOfView        float32
	// world units; nil, half a cell everywhere
	Fine func(x, y float32) (step, thin float32)
	// nil, under the ground alone
	Solid func(x, y, z float32) (top float32, in bool)
}

// Start is how a camera begins: FromAbove straight down, a screen unit a world unit, or
// Isometrically as the 2:1 view, a cell a TileW-wide diamond.
type Start uint8

const (
	FromAbove Start = iota
	Isometrically
)

// Maker makes a world's cameras beginning as start says, over ground (its top as drawn at a
// point) between the heights extent gives (nil: level at sea level), the ground far off sinking
// bend per distance² under a perspective eye's level. It refuses a wrapping world.
func Maker(
	cfg Config,
	start Start,
	ground func(x, y float32) float32,
	extent func() (low, high float32),
	bend float32,
) func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
	o := optics{
		Cell:       cfg.Cell,
		TileW:      cfg.TileW,
		TileH:      cfg.TileH,
		HeightUnit: cfg.HeightUnit,
		Headroom:   cfg.Headroom,
		MinPitch:   cfg.MinPitch * math.Pi / 180,
	}.withDefaults()
	return func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
		cam := newCamera(o, start, width, height, edges, c, cfg.FieldOfView*math.Pi/180,
			cfg.Perspective, ground, extent, bend)
		cam.ortho.fine, cam.persp.fine = cfg.Fine, cfg.Fine
		cam.ortho.solid, cam.persp.solid = cfg.Solid, cfg.Solid
		return cam
	}
}

// Switch has cam, a camera Maker made, switch its projection as Tab does, and is false for any
// other camera.
func Switch(cam camera.Camera) bool {
	c, ok := cam.(*viewCamera)
	if ok {
		c.next()
	}
	return ok
}

// Turn turns cam, a camera Maker made, by angle radians as Q and E do, and is false for any
// other camera.
func Turn(cam camera.Camera, angle float32) bool {
	c, ok := cam.(*viewCamera)
	if ok {
		c.Turn(angle)
	}
	return ok
}

// Tilt tilts cam, a camera Maker made, by angle radians, towards straight down as it grows, as R
// and F do, and is false for any other camera.
func Tilt(cam camera.Camera, angle float32) bool {
	c, ok := cam.(*viewCamera)
	if ok {
		c.Tilt(angle)
	}
	return ok
}
