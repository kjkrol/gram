package topography

import "github.com/kjkrol/gram/camera"

// TurnCamera turns cam, a camera of the topography's, by angle.
func TurnCamera(cam camera.Camera, angle float32) { cam.(*viewCamera).Turn(angle) }

// SwitchView has cam, a camera of the topography's, look the next way round: from above,
// isometrically, in perspective where reached.
func SwitchView(cam camera.Camera) { cam.(*viewCamera).next() }

// InPerspective reports whether cam, a camera of the topography's, is in its perspective view.
func InPerspective(cam camera.Camera) bool { return cam.(*viewCamera).inPersp }
