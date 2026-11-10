// Package cameratest is the cameras plugin's cameras for tests: one from above to draw or cull
// through when the camera itself is not under test, and a relief camera's keys.
package cameratest

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/internal/stick"
	"github.com/kjkrol/gram/plugins/cameras/internal/topdown"
)

// NewFromSpace is a camera from above over a width x height world, viewing the window viewport
// gives, all of it when none.
func NewFromSpace(
	width, height uint32,
	edges aabbworld.Edges,
	viewport ...camera.AABB,
) camera.Camera {
	return topdown.NewFromSpace(width, height, edges, viewport...)
}

// NewFromAbove is a camera of a Config seeing in relief over a width x height world of level
// ground, a cell 32 across, looking straight down: one to Turn and Tilt.
func NewFromAbove(width, height uint32) camera.Camera {
	made := stick.Maker(stick.Config{Cell: 32}, stick.FromAbove, nil, nil, 0)
	return made(width, height, 0, camera.Config{})
}

// Switch has cam, a camera of a Config seeing in relief, switch its projection as View does, and
// is false for any other camera.
func Switch(cam camera.Camera) bool { return stick.Switch(cam) }

// Turn turns cam, a camera of a Config seeing in relief, by angle radians as cameras.Turn does,
// and is false for any other camera.
func Turn(cam camera.Camera, angle float32) bool { return stick.Turn(cam, angle) }

// Tilt tilts cam, a camera of a Config seeing in relief, by angle radians as cameras.Tilt does,
// and is false for any other camera.
func Tilt(cam camera.Camera, angle float32) bool { return stick.Tilt(cam, angle) }

// Showing is cam with a picture showing every box, the rest as cam has it: for a test of what
// rests on the scale and the distance alone, which the picture would bound.
func Showing(cam camera.Camera) camera.Camera { return showing{cam} }

type showing struct{ camera.Camera }

var (
	_ camera.Focused = showing{}
	_ camera.Sighted = showing{}
	_ camera.Shifted = showing{}
	_ camera.Scaler  = showing{}
)

func (showing) Visible(camera.AABB) bool { return true }

func (s showing) Focus() (x, y, z float32) {
	if f, ok := s.Camera.(camera.Focused); ok {
		return f.Focus()
	}
	box := s.Bounds()
	return (box.TopLeft.X + box.BottomRight.X) / 2, (box.TopLeft.Y + box.BottomRight.Y) / 2, 0
}

func (s showing) Sight() float32                  { return camera.SightOf(s.Camera) }
func (s showing) LevelShift() lod.Level           { return camera.LevelShiftOf(s.Camera) }
func (s showing) ScaleAt(x, y, z float32) float32 { return camera.ScaleAt(s.Camera, x, y, z) }

func (s showing) See(factor float32) {
	if c, ok := s.Camera.(interface{ See(float32) }); ok {
		c.See(factor)
	}
}

func (s showing) SetLevelShift(n lod.Level) {
	if c, ok := s.Camera.(camera.Shifted); ok {
		c.SetLevelShift(n)
	}
}
