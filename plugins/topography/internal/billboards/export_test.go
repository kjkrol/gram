package billboards

import (
	"math"

	"github.com/kjkrol/gram/plugins/world"
)

// Stood is every billboard the look l took since it was readied: its centre, its altitude, how
// tall and how wide it stands, and how far its top leans.
func Stood(l world.Look) [][6]float32 {
	var out [][6]float32
	for _, b := range l.(Look).gpu.batches {
		for i := 0; i+boardFloats <= len(b.inst); i += boardFloats {
			in := b.inst[i : i+boardFloats]
			out = append(out, [6]float32{in[0], in[1], in[2], in[3], in[4], float32(math.Hypot(float64(in[5]), float64(in[6])))})
		}
	}
	return out
}

// Lain is the corners (x, y, height) of every sprite the look l took lying since it was readied.
func Lain(l world.Look) [][3]float32 {
	var out [][3]float32
	for _, b := range l.(Look).gpu.lain {
		for i := 0; i+lainFloats <= len(b.inst); i += lainFloats {
			for k := range 4 {
				in := b.inst[i+4*k:]
				out = append(out, [3]float32{in[0], in[1], in[2]})
			}
		}
	}
	return out
}

// Cast is every shadow the look l cast since it was readied: a patch's sprite corners all 0.
func Cast(l world.Look) [][castFloats]float32 {
	var out [][castFloats]float32
	for _, b := range l.(Look).gpu.casts {
		for i := 0; i+castFloats <= len(b.inst); i += castFloats {
			out = append(out, [castFloats]float32(b.inst[i:i+castFloats]))
		}
	}
	return out
}
