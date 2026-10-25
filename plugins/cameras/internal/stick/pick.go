package stick

import (
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/vec"
)

// bend lifts the line by bend·h² at h along the ground off o: the Earth's curve as the eye sees it.
type sight struct {
	o, d [3]float32
	bend float32
}

// world units along the line
const pickPrecision = 1e-3

func (s sight) at(t float32) (x, y, z float32) {
	h2 := (s.d[0]*s.d[0] + s.d[1]*s.d[1]) * t * t
	return s.o[0] + s.d[0]*t, s.o[1] + s.d[1]*t, s.o[2] + s.d[2]*t + s.bend*h2
}

func (s sight) rising(t float32) bool {
	return s.d[2]+2*s.bend*(s.d[0]*s.d[0]+s.d[1]*s.d[1])*t > 0
}

func (s sight) pick(solid func(x, y, z float32) (float32, bool),
	fine func(x, y float32) (float32, float32), t1, step, high float32) (float32, float32, float32,
	bool) {
	n := vec.Norm(s.d)
	into := func(t float32) (float32, bool) {
		x, y, z := s.at(t)
		return solid(x, y, z)
	}
	if top, in := into(0); n == 0 || t1 <= 0 || in {
		return s.o[0], s.o[1], top, n != 0 && t1 > 0
	}
	dt := step / n
	fall := f32.Abs(s.d[2])
	lo := float32(0)
	for {
		hi := min(lo+dt, t1)
		if fine != nil {
			x0, y0, _ := s.at(lo)
			x1, y1, _ := s.at(hi)
			f0, thin0 := fine(x0, y0)
			f1, thin1 := fine(x1, y1)
			if f := max(f0, f1); f > 0 && f < step {
				hi = min(lo+f/n, hi)
			}
			if thin := max(thin0, thin1); thin > 0 && fall > 0 {
				hi = min(lo+thin/2/fall, hi)
			}
		}
		if top, in := into(hi); in {
			for k := 0; k < 48 && (hi-lo)*n > pickPrecision; k++ {
				mid := (lo + hi) / 2
				if t, in := into(mid); in {
					hi, top = mid, t
				} else {
					lo = mid
				}
			}
			x, y, _ := s.at(hi)
			return x, y, top, true
		}
		if _, _, z := s.at(hi); hi >= t1 || z > high && s.rising(hi) {
			break
		}
		lo = hi
	}
	x, y, z := s.at(t1)
	return x, y, z, false
}

func grounded(top func(x, y float32) float32) func(x, y, z float32) (float32, bool) {
	return func(x, y, z float32) (float32, bool) {
		t := top(x, y)
		return t, z < t
	}
}
