package air

import (
	"math"

	"github.com/kjkrol/gram/camera"
)

// Haze is how much of what lies at the world point (x, y, z) w's air hides from the eye of cam,
// 0 to 1: 1 − e^(−d/v), d its distance from the eye (camera.Eyed) and v the Visibility; 0
// through a camera without an eye or in air without end. What is drawn hazed turns to the Fog
// (render.Frame.Fog), the sky under the clouds (Frame).
func (w Weather) Haze(cam camera.Camera, x, y, z float32) float32 {
	v := w.Visibility
	if v <= 0 || cam == nil {
		return 0
	}
	e, ok := cam.(camera.Eyed)
	if !ok {
		return 0
	}
	ex, ey, ez, ok := e.Eye()
	if !ok {
		return 0
	}
	d := math.Sqrt(float64((x-ex)*(x-ex) + (y-ey)*(y-ey) + (z-ez)*(z-ez)))
	return float32(1 - math.Exp(-d/v))
}
