package cameras

import "github.com/kjkrol/gram/plugins/topography/internal/vec"

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

// pick is the first point along s, between 0 and t1, where the line passes under top: stepped
// step world units at a time, then halved. False where it passes under nothing before t1, or
// climbs past high.
func (s sight) pick(top func(x, y float32) float32, t1, step, high float32) (float32, float32, bool) {
	n := vec.Norm(s.d)
	under := func(t float32) bool {
		x, y, z := s.at(t)
		return z < top(x, y)
	}
	if n == 0 || t1 <= 0 || under(0) {
		return s.o[0], s.o[1], n != 0 && t1 > 0
	}
	dt := step / n
	lo := float32(0)
	for i := 1; ; i++ {
		hi := min(float32(i)*dt, t1)
		if under(hi) {
			for k := 0; k < 48 && (hi-lo)*n > pickPrecision; k++ {
				if mid := (lo + hi) / 2; under(mid) {
					hi = mid
				} else {
					lo = mid
				}
			}
			x, y, _ := s.at(hi)
			return x, y, true
		}
		if _, _, z := s.at(hi); hi >= t1 || z > high && s.rising(hi) {
			break
		}
		lo = hi
	}
	x, y, _ := s.at(t1)
	return x, y, false
}
