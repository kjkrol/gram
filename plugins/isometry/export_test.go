package isometry

import "github.com/kjkrol/gram/camera"

// TurnCamera turns cam, an isometric camera, by angle.
func TurnCamera(cam camera.Camera, angle float32) { cam.(*isoCamera).Turn(angle) }
