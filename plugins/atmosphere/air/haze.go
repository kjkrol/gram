package air

import (
	"math"

	"github.com/kjkrol/gram/camera"
)

// Haze is how much of what lies at the world point (x, y, z) the air over the eye of cam hides
// from it, 0 to 1; 0 through a camera without an eye or in air without end.
func (w Weather) Haze(cam camera.Camera, x, y, z float32) float32 {
	return w.Hazer(cam).At(x, y, z)
}

// Hazer is the air's haze through a camera, worked out once for everything one frame draws.
type Hazer struct {
	eye   [3]float32
	sight float64 // how far one sees from the eye; 0 for no haze
}

// Hazer is w's haze through cam (Haze).
func (w Weather) Hazer(cam camera.Camera) Hazer {
	if w.Sight <= 0 || cam == nil {
		return Hazer{}
	}
	e, ok := cam.(camera.Eyed)
	if !ok {
		return Hazer{}
	}
	ex, ey, ez, ok := e.Eye()
	if !ok {
		return Hazer{}
	}
	return Hazer{eye: [3]float32{ex, ey, ez}, sight: w.Visibility(ex, ey)}
}

// At is how much of what lies at the world point (x, y, z) the air hides from the eye, 0 to 1.
func (h Hazer) At(x, y, z float32) float32 {
	if h.sight <= 0 {
		return 0
	}
	dx, dy, dz := x-h.eye[0], y-h.eye[1], z-h.eye[2]
	return float32(1 - math.Exp(-math.Sqrt(float64(dx*dx+dy*dy+dz*dz))/h.sight))
}

// Under is the world point the air is felt at through cam: under its eye, or in the middle of its
// view for a camera without one.
func Under(cam camera.Camera) (x, y float32) {
	if e, ok := cam.(camera.Eyed); ok {
		if ex, ey, _, ok := e.Eye(); ok {
			return ex, ey
		}
	}
	w, h := cam.Viewport()
	return cam.Unproject(w/2, h/2, 0)
}
