package f32_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/internal/f32"
)

func TestEps_SpansSomeStepsOfFloat32AtItsScale(t *testing.T) {
	for _, x := range []float32{0, 0.5, 1, 155.1397, 6528, 16384, 98000} {
		step := math.Nextafter32(x, float32(math.Inf(1))) - x
		if e := f32.Eps(x); e < 4*step || e < 1e-6 {
			t.Errorf("Eps(%v) = %v, want at least four steps of %v and 1e-6", x, e, step)
		}
		if e := f32.Eps(-x); e != f32.Eps(x) {
			t.Errorf("Eps(%v) = %v, want Eps(%v) = %v", -x, e, x, f32.Eps(x))
		}
	}
}
