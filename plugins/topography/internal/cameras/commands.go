package cameras

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
)

// LookStep is how far a pixel of the mouse turns the eye riding in an entity: a seventh of a
// degree.
const LookStep = 0.0025

// Orders are the commands a player gives the cameras, as the topography queues them: each method
// hands fn every one given since, in order, and forgets them. View has the camera look the next
// way round; Turn turns it, Tilt bows its head; LookFrom and LookAt put its eye in perspective;
// LookOut has it ride in the selected unit, first person; Follow fastens it behind the unit; Drive
// steers the unit it is fastened to; Look turns the eye riding in a unit, Dx and Dy pixels.
type Orders interface {
	Views(fn func(cam camera.Camera))
	Turns(fn func(cam camera.Camera, angle float32))
	Tilts(fn func(cam camera.Camera, angle float32))
	LookFroms(fn func(cam camera.Camera, x, y, z float32))
	LookAts(fn func(cam camera.Camera, x, y, z float32))
	LookOuts(fn func(cam camera.Camera, by control.PlayerID))
	Follows(fn func(cam camera.Camera, by control.PlayerID))
	Drives(fn func(cam camera.Camera, ahead, turn int8, sprint bool))
	Looks(fn func(cam camera.Camera, dx, dy float32))
}

// Control carries out the view's commands — View, Turn, Tilt, Follow, Drive, LookFrom, LookAt,
// LookOut, Look, as the topography's Orders give them — on the cameras as its System runs: over the
// ground of a relief, the top of the cell under a point as topAt says; where
// the game reaches the perspective; given the selection, fastening a camera to the one selected
// unit.
type Control struct {
	relief      *relief.Relief
	topAt       func(geom.Vec) float64
	perspective bool
	selected    tag.Tag[selection.Family]
	selecting   bool
}

// NewControl is the control of the cameras over ground, its top under a point as topAt says (nil:
// the ground), reaching the perspective where perspective says.
func NewControl(ground *relief.Relief, topAt func(geom.Vec) float64, perspective bool) *Control {
	return &Control{relief: ground, topAt: topAt, perspective: perspective}
}

// WithSelection has Follow and LookOut fasten a camera to the one entity carrying selected.
func (ctl *Control) WithSelection(selected tag.Tag[selection.Family]) *Control {
	ctl.selected, ctl.selecting = selected, true
	return ctl
}

// System is the control as a goke.System, run once a tick in the interface part of a plan,
// carrying out what orders gives it.
func (ctl *Control) System(orders Orders) goke.System {
	return &cameraSystem{orders: orders, relief: ctl.relief, topAt: ctl.topAt, selected: ctl.selected, selecting: ctl.selecting}
}
