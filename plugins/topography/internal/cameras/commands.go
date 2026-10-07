package cameras

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
)

// LookStep is how far a pixel of the mouse turns the eye riding in an entity: a seventh of a
// degree.
const LookStep = 0.0025

// Orders are the commands a player gives the cameras, as the topography queues them: each method
// hands fn every one given since, in order, and forgets them. View has the camera look the next
// way round; Turn turns it, Tilt bows its head; LookFrom and LookAt put its eye in perspective;
// Ride takes it closer round the entity it is fastened to — behind it, inside it, over it again;
// Look turns the eye riding in a unit, Dx and Dy pixels.
type Orders interface {
	Views(fn func(cam camera.Camera))
	Turns(fn func(cam camera.Camera, angle float32))
	Tilts(fn func(cam camera.Camera, angle float32))
	LookFroms(fn func(cam camera.Camera, x, y, z float32))
	LookAts(fn func(cam camera.Camera, x, y, z float32))
	Rides(fn func(cam camera.Camera))
	Looks(fn func(cam camera.Camera, dx, dy float32))
}

// Control carries out the view's commands — View, Turn, Tilt, Ride, LookFrom, LookAt, Look, as
// the topography's Orders give them — on the cameras its Maker made, as its System runs: over
// the ground of a relief, the top of the cell under a point as topAt says; where the game
// reaches the perspective.
type Control struct {
	relief      *relief.Relief
	topAt       func(geom.Vec) float64
	perspective bool
	cams        []*viewCamera // every camera the Maker made: what the system keeps fastened
}

// NewControl is the control of the cameras over ground, its top under a point as topAt says (nil:
// the ground), reaching the perspective where perspective says.
func NewControl(ground *relief.Relief, topAt func(geom.Vec) float64, perspective bool) *Control {
	return &Control{relief: ground, topAt: topAt, perspective: perspective}
}

// Maker is Maker, every camera it makes noted by the control, whose system keeps it fastened.
func (ctl *Control) Maker(cfg Config, ground func(x, y float32) float32, extent func() (low, high float32), bend float32) func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
	make := Maker(cfg, ground, extent, bend)
	return func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
		cam := make(width, height, edges, c)
		ctl.cams = append(ctl.cams, cam.(*viewCamera))
		return cam
	}
}

// System is the control as a goke.System, run once a tick in the interface part of a plan,
// carrying out what orders gives it.
func (ctl *Control) System(orders Orders) goke.System {
	return &cameraSystem{orders: orders, relief: ctl.relief, topAt: ctl.topAt, perspective: ctl.perspective, cams: &ctl.cams}
}
