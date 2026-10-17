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

// Seen is how the air hazes the ground cam sees, for its shaders: how far one sees (seeing, 0
// for no haze), from an eye by the distance from it; through a camera looking from above, which
// has none, only if fromAbove, by the air over the ground up to the high clouds' base (up) along
// its lines of sight (way).
func (w Weather) Seen(cam camera.Camera, fromAbove bool) (seeing, up float32, way [3]float32) {
	if w.Sight <= 0 || cam == nil {
		return 0, 0, way
	}
	if e, ok := cam.(camera.Eyed); ok {
		if _, _, _, ok := e.Eye(); ok {
			return float32(w.Visibility(Under(cam))), 0, way
		}
	}
	rays, ok := cam.(camera.Rays)
	if !fromAbove || !ok {
		return 0, 0, way
	}
	f, ok := rays.Rays()
	n := float32(math.Sqrt(float64(f.Dir[0]*f.Dir[0] + f.Dir[1]*f.Dir[1] + f.Dir[2]*f.Dir[2])))
	if !ok || n == 0 || f.DDX != ([3]float32{}) || f.DDY != ([3]float32{}) {
		return 0, 0, way // a perspective without an eye: none to haze from
	}
	way = [3]float32{f.Dir[0] / n, f.Dir[1] / n, f.Dir[2] / n}
	return float32(w.Visibility(Under(cam))), w.Base(High), way
}
