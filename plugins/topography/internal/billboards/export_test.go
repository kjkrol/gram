package billboards

import (
	"math"

	"github.com/kjkrol/gram/plugins/world"
)

// Stood is what the look l took for the GPU since it was readied: for every billboard its centre,
// its altitude, how tall and how wide it stands, and how far its top leans.
func Stood(l world.Look) [][6]float32 {
	var out [][6]float32
	for _, b := range l.(Look).gpu.batches {
		for i := 0; i+16 <= len(b.inst); i += 16 {
			in := b.inst[i : i+16]
			out = append(out, [6]float32{in[0], in[1], in[2], in[3], in[4], float32(math.Hypot(float64(in[5]), float64(in[6])))})
		}
	}
	return out
}
