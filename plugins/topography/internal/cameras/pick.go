package cameras

import (
	"math"

	"github.com/kjkrol/gram/plugins/topography/internal/vec"
)

// sight is a line of sight from o along d: t along it, it stands at o[2] + d[2]·t, lifted by
// bend·h² over the ground h off o, the ground's curve as the eye at o sees it.
type sight struct {
	o, d [3]float32
	bend float32
}

// pickPrecision is how near, in world units along the line, pick finds where it meets the ground.
const pickPrecision = 1e-3

// at is the ground point t along s and the height the line stands at over it.
func (s sight) at(t float32) (x, y, z float32) {
	h2 := (s.d[0]*s.d[0] + s.d[1]*s.d[1]) * t * t
	return s.o[0] + s.d[0]*t, s.o[1] + s.d[1]*t, s.o[2] + s.d[2]*t + s.bend*h2
}

// rising reports whether the line climbs at t.
func (s sight) rising(t float32) bool {
	return s.d[2]+2*s.bend*(s.d[0]*s.d[0]+s.d[1]*s.d[1])*t > 0
}

// pick is the first point along s, between 0 and t1, where the line passes into what solid says
// is solid there — under the ground, inside a deck's slab — and the top of what it found: stepped
// step world units at a time, then halved. False where it passes into nothing before t1, or climbs
// past high. Where fine says a deck lies near — narrower than step, or its slab thinner than a
// step falls — it steps that fine there.
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
	fall := float32(math.Abs(float64(s.d[2])))
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

// grounded is solid as the ground top gives it: everything under it.
func grounded(top func(x, y float32) float32) func(x, y, z float32) (float32, bool) {
	return func(x, y, z float32) (float32, bool) {
		t := top(x, y)
		return t, z < t
	}
}
