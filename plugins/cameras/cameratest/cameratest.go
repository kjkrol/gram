// Package cameratest is the cameras plugin's cameras for tests: one from above to draw or cull
// through when the camera itself is not under test, and a way round a relief camera's views.
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

// Switch has cam, a camera of a Config seeing in relief, look the next way round as View does,
// and is false for any other camera.
func Switch(cam camera.Camera) bool { return stick.Switch(cam) }
