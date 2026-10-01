package cameras

import (
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
)

// Config is the views of a map in relief: the isometric view — a Cell-sized square of the world a
// TileW x TileH diamond, a height lifting a point HeightUnit screen units per world unit, Headroom
// how far above the ground a camera looks for sprites; zero TileW, TileH, HeightUnit and Headroom
// are 64, 32, 1 and 64, Cell must be set — whether a camera begins Isometric rather than from above,
// how flat the eye may look along the ground (MinPitch, degrees; 30 when zero), and whether View
// reaches a third view, in Perspective, seeing FieldOfView degrees from the top of the screen to the
// bottom (45 when zero).
type Config struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
	Isometric          bool
	MinPitch           float32
	Perspective        bool
	FieldOfView        float32
}

// Maker makes a world's cameras as cfg says (world.Plugin.SetCameras): over ground — its top as it
// is drawn at a point — between the heights extent gives (nil: level at sea level), the ground far
// off sinking bend per distance² under a perspective eye's level. It refuses a wrapping world.
func Maker(cfg Config, ground func(x, y float32) float32, extent func() (low, high float32), bend float32) func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
	proj := projection{Cell: cfg.Cell, TileW: cfg.TileW, TileH: cfg.TileH, HeightUnit: cfg.HeightUnit, Headroom: cfg.Headroom,
		MinPitch: cfg.MinPitch * math.Pi / 180, flat: !cfg.Isometric}.withDefaults()
	return func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
		return newCamera(proj, width, height, edges, c, cfg.FieldOfView*math.Pi/180, cfg.Perspective, ground, extent, bend)
	}
}

// Switch has cam, a camera Maker made, look the next way round at once, as View does from its
// queue: from above, isometrically, in perspective where reached, from above again; an eye inside
// a unit comes out instead. It is false for any other camera.
func Switch(cam camera.Camera) bool {
	c, ok := cam.(*viewCamera)
	if ok {
		c.next()
	}
	return ok
}
