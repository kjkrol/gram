package entity

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Velocity is an entity's heading (Dir, a unit vector, zero when stationary), speed (Value,
// world units a second; under zero it backs away, still facing Dir) and Side, its speed across
// Dir, to its right (the screen's y down) over zero: a strafe, still facing Dir.
type Velocity struct {
	Dir   geom.Vec
	Value float64
	Side  float64
}

// Delta returns Velocity's current per-axis rate: Dir scaled by Value, and Side across it.
func (v Velocity) Delta() geom.Vec {
	if v.Side == 0 {
		return geom.NewVec(v.Dir.X*v.Value, v.Dir.Y*v.Value)
	}
	return geom.NewVec(v.Dir.X*v.Value-v.Dir.Y*v.Side, v.Dir.Y*v.Value+v.Dir.X*v.Side)
}

// SetDelta sets Dir and Value from a per-axis rate; an entity backing away (Value under zero)
// goes on backing, facing against the rate.
func (v *Velocity) SetDelta(d geom.Vec) {
	v.Side = 0
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

// Slow keeps Dir and takes of a per-axis rate only what runs along it, never past a stop.
func (v *Velocity) Slow(d geom.Vec) {
	v.Side = 0
	along := d.X*v.Dir.X + d.Y*v.Dir.Y
	if v.Value < 0 {
		v.Value = min(along, 0)
		return
	}
	v.Value = max(along, 0)
}
