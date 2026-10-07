package cameras

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// The commands as the topography gives them, for the tests to queue.
type (
	View struct{ Camera camera.Camera }
	Ride struct{ Camera camera.Camera }
	Turn struct {
		Camera camera.Camera
		Angle  float32
	}
	Tilt struct {
		Camera camera.Camera
		Angle  float32
	}
	LookFrom struct {
		Camera  camera.Camera
		X, Y, Z float32
	}
	LookAt struct {
		Camera  camera.Camera
		X, Y, Z float32
	}
	Look struct {
		Camera camera.Camera
		Dx, Dy float32
	}
)

// queued is Orders over the tests' queues, as the topography's are over its own; a nil queue gives
// nothing.
type queued struct {
	views     *control.Queue[View]
	turns     *control.Queue[Turn]
	tilts     *control.Queue[Tilt]
	lookFroms *control.Queue[LookFrom]
	lookAts   *control.Queue[LookAt]
	rides     *control.Queue[Ride]
	looks     *control.Queue[Look]
}

var _ Orders = queued{}

func drain[C any](q *control.Queue[C], fn func(control.Issued[C])) {
	if q != nil {
		q.Drain(fn)
	}
}

func (q queued) Views(fn func(camera.Camera)) {
	drain(q.views, func(i control.Issued[View]) { fn(i.Command.Camera) })
}

func (q queued) Turns(fn func(camera.Camera, float32)) {
	drain(q.turns, func(i control.Issued[Turn]) { fn(i.Command.Camera, i.Command.Angle) })
}

func (q queued) Tilts(fn func(camera.Camera, float32)) {
	drain(q.tilts, func(i control.Issued[Tilt]) { fn(i.Command.Camera, i.Command.Angle) })
}

func (q queued) LookFroms(fn func(camera.Camera, float32, float32, float32)) {
	drain(q.lookFroms, func(i control.Issued[LookFrom]) { fn(i.Command.Camera, i.Command.X, i.Command.Y, i.Command.Z) })
}

func (q queued) LookAts(fn func(camera.Camera, float32, float32, float32)) {
	drain(q.lookAts, func(i control.Issued[LookAt]) { fn(i.Command.Camera, i.Command.X, i.Command.Y, i.Command.Z) })
}

func (q queued) Rides(fn func(camera.Camera)) {
	drain(q.rides, func(i control.Issued[Ride]) { fn(i.Command.Camera) })
}

func (q queued) Looks(fn func(camera.Camera, float32, float32)) {
	drain(q.looks, func(i control.Issued[Look]) { fn(i.Command.Camera, i.Command.Dx, i.Command.Dy) })
}
