package entity

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Velocity is an entity's heading (Dir, a unit vector, zero when stationary)
// and speed (Value, world units a second; under zero it backs away, still facing Dir).
type Velocity struct {
	Dir   geom.Vec
	Value float64
}

// Delta returns Velocity's current per-axis rate (Dir scaled by Value).
func (v Velocity) Delta() geom.Vec {
	return geom.NewVec(v.Dir.X*v.Value, v.Dir.Y*v.Value)
}

// SetDelta sets Dir and Value from a per-axis rate; an entity backing away (Value under zero)
// goes on backing, facing against the rate.
func (v *Velocity) SetDelta(d geom.Vec) {
	mag := math.Hypot(d.X, d.Y)
	if mag < 1e-9 {
		v.Value = 0
		return
	}
	if v.Value < 0 {
		v.Dir = geom.NewVec(-d.X/mag, -d.Y/mag)
		v.Value = -mag
		return
	}
	v.Dir = geom.NewVec(d.X/mag, d.Y/mag)
	v.Value = mag
}
