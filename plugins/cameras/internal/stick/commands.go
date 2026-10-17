package stick

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
)

// LookStep is how far a pixel of the mouse turns the eye riding in an entity, in radians: a
// seventh of a degree.
const LookStep = 0.0025

// Orders are the commands a player gives the cameras, as the cameras plugin queues them: each
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

// Control carries out the views' commands on the cameras its Maker made, through its Keeper.
type Control struct {
	free        func(x, y, z float32) bool
	perspective bool
	cams        []*viewCamera
}

// NewControl is the control of the cameras in a space where free says nothing solid fills a
// point (nil: everywhere), reaching the perspective where perspective says.
func NewControl(free func(x, y, z float32) bool, perspective bool) *Control {
	return &Control{free: free, perspective: perspective}
}

// Maker is Maker, every camera it makes kept fastened by the control's Keeper.
func (ctl *Control) Maker(
	cfg Config,
	ground func(x, y float32) float32,
	extent func() (low, high float32),
	bend float32,
) func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
	make := Maker(cfg, ground, extent, bend)
	return func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
		cam := make(width, height, edges, c)
		ctl.cams = append(ctl.cams, cam.(*viewCamera))
		return cam
	}
}

// Keeper is the control carrying out what orders gives it.
func (ctl *Control) Keeper(orders Orders) *Keeper {
	return &Keeper{
		orders:      orders,
		free:        ctl.free,
		perspective: ctl.perspective,
		cams:        &ctl.cams,
	}
}
