package cameras

import (
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
)

// Config is the views of a map in relief: a Cell-sized square of the world drawn isometrically
// as a TileW x TileH diamond (64 x 32 when zero), a height lifting a point HeightUnit screen
// units a world unit (1), Headroom how far above the ground a camera looks for sprites (64),
// whether a camera begins Isometric, MinPitch the flattest the eye may look in degrees (30), and
// whether View reaches a Perspective seeing FieldOfView degrees top to bottom (45). Cell must
// be set.
type Config struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
	Isometric          bool
	MinPitch           float32
	Perspective        bool
	FieldOfView        float32
	// radians a pixel of a pan the world's edge stopped; zero, neither
	EdgeTurn, EdgeTilt float32
	// radians a pixel of a scroll from a corner of the window; zero, a corner scrolls
	CornerTurn float32
	// world units; nil, half a cell everywhere
	Fine func(x, y float32) (step, thin float32)
	// nil, under the ground alone
	Solid func(x, y, z float32) (top float32, in bool)
}

// Maker makes a world's cameras as cfg says, over ground (its top as drawn at a point) between
// the heights extent gives (nil: level at sea level), the ground far off sinking bend per
// distance² under a perspective eye's level. It refuses a wrapping world.
func Maker(cfg Config, ground func(x, y float32) float32, extent func() (low, high float32), bend float32) func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
	proj := projection{Cell: cfg.Cell, TileW: cfg.TileW, TileH: cfg.TileH, HeightUnit: cfg.HeightUnit, Headroom: cfg.Headroom,
		MinPitch: cfg.MinPitch * math.Pi / 180, flat: !cfg.Isometric}.withDefaults()
	return func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
		cam := newCamera(proj, width, height, edges, c, cfg.FieldOfView*math.Pi/180,
			cfg.Perspective, ground, extent, bend)
		cam.edgeTurn, cam.edgeTilt, cam.cornerTurn = cfg.EdgeTurn, cfg.EdgeTilt, cfg.CornerTurn
		cam.iso.fine, cam.persp.fine = cfg.Fine, cfg.Fine
		cam.iso.solid, cam.persp.solid = cfg.Solid, cfg.Solid
		return cam
	}
}

// Switch has cam, a camera Maker made, look the next way round as View does, and is false for
// any other camera.
func Switch(cam camera.Camera) bool {
	c, ok := cam.(*viewCamera)
	if ok {
		c.next()
	}
	return ok
}
