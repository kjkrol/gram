// Package cameratest is the cameras plugin's cameras for tests: one from above to draw or cull
// through when the camera itself is not under test, and a relief camera's keys.
package cameratest

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/camera"
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
