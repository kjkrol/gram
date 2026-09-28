package entity

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Velocity is an entity's heading (Dir, a unit vector, zero when stationary)
// and speed (Value, world units a second).
type Velocity struct {
	Dir   geom.Vec
	Value float64
}

// Delta returns Velocity's current per-axis rate (Dir scaled by Value).
func (v Velocity) Delta() geom.Vec {
	return geom.NewVec(v.Dir.X*v.Value, v.Dir.Y*v.Value)
}

// SetDelta sets Dir and Value from a per-axis rate.
func (v *Velocity) SetDelta(d geom.Vec) {
	mag := math.Hypot(d.X, d.Y)
	if mag < 1e-9 {
		v.Value = 0
		return
	}
	v.Dir = geom.NewVec(d.X/mag, d.Y/mag)
	v.Value = mag
}
