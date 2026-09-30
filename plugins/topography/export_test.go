package topography

import (
	"math"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
)

// TurnCamera turns cam, a camera of the topography's, by angle.
func TurnCamera(cam camera.Camera, angle float32) { cam.(*viewCamera).Turn(angle) }

// SwitchView has cam, a camera of the topography's, look the next way round: from above,
// isometrically, in perspective where reached.
func SwitchView(cam camera.Camera) { cam.(*viewCamera).next() }

// InPerspective reports whether cam, a camera of the topography's, is in its perspective view.
func InPerspective(cam camera.Camera) bool { return cam.(*viewCamera).inPersp }

// FillRun writes into h the run of r's heights from h.First on.
func FillRun(r *Relief, h *Heights) { r.fill(h) }

// AdoptRuns has r take the runs' heights as the ground, as a loaded game's.
func AdoptRuns(r *Relief, runs []Heights) bool { return r.adopt(runs) }

// Billboards is what the topography's look l took for the GPU since it was readied: for every
// billboard its centre, its altitude, how tall and how wide it stands, and how far its top leans.
func Billboards(l world.Look) [][6]float32 {
	var out [][6]float32
	for _, b := range l.(worldLook).gpu.batches {
		for i := 0; i+16 <= len(b.inst); i += 16 {
			in := b.inst[i : i+16]
			out = append(out, [6]float32{in[0], in[1], in[2], in[3], in[4], float32(math.Hypot(float64(in[5]), float64(in[6])))})
		}
	}
	return out
}
