package cameras

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
)

// LookStep is how far a pixel of the mouse turns the eye riding in an entity, in radians: a
// seventh of a degree.
const LookStep = 0.0025

// Orders are the commands a player gives the cameras, as the topography queues them: each
// method hands fn every one given since, in order, and forgets them.
type Orders interface {
	Views(fn func(cam camera.Camera))
	Turns(fn func(cam camera.Camera, angle float32))
	Tilts(fn func(cam camera.Camera, angle float32))
	LookFroms(fn func(cam camera.Camera, x, y, z float32))
	LookAts(fn func(cam camera.Camera, x, y, z float32))
	Rides(fn func(cam camera.Camera))
	Looks(fn func(cam camera.Camera, dx, dy float32))
}

// Control carries out the views' commands on the cameras its Maker made, as its System runs.
type Control struct {
	relief      *relief.Relief
	topAt       func(geom.Vec) float64
	perspective bool
	cams        []*viewCamera
}

// NewControl is the control of the cameras over ground, the top of the cell under a point as
// topAt says (nil: the ground), reaching the perspective where perspective says.
func NewControl(ground *relief.Relief, topAt func(geom.Vec) float64, perspective bool) *Control {
	return &Control{relief: ground, topAt: topAt, perspective: perspective}
}

// Maker is Maker, every camera it makes kept fastened by the control's system.
func (ctl *Control) Maker(cfg Config, ground func(x, y float32) float32, extent func() (low, high float32), bend float32) func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
	make := Maker(cfg, ground, extent, bend)
	return func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
		cam := make(width, height, edges, c)
		ctl.cams = append(ctl.cams, cam.(*viewCamera))
		return cam
	}
}

// System is the control as a goke.System, carrying out what orders gives it.
func (ctl *Control) System(orders Orders) goke.System {
	return &cameraSystem{orders: orders, relief: ctl.relief, topAt: ctl.topAt, perspective: ctl.perspective, cams: &ctl.cams}
}
