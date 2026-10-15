// Package planar is a body's fall across the plane, a platformer's: the pull any way across it,
// the body borne by whatever collision found it touching against the pull.
package planar

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// a surface bears a body where n·ĝ is under −bears: a slope of up to some 45°, never a wall
const bears = 0.7

// Body is one body as a step takes it: the drift it is carried at and whether it is in the air.
type Body struct {
	Drift    geom.Vec
	Airborne bool
}

// Settle has the surfaces the body touches (their normals, collision's) take the drift that runs
// into them, one facing against the pull g bearing it. A body coming down out of the air lands,
// at the speed it ran into the surface.
func Settle(b *Body, g geom.Vec, normals []geom.Vec) (speed float64, landed bool) {
	down := unit(g)
	borne := false
	for _, n := range normals {
		into := dot(b.Drift, n)
		if into < 0 {
			b.Drift = geom.NewVec(b.Drift.X-into*n.X, b.Drift.Y-into*n.Y)
		}
		if dot(n, down) < -bears && into <= 0 {
			speed, borne = max(speed, -into), true
		}
	}
	landed = borne && b.Airborne
	b.Airborne = !borne
	if !landed {
		speed = 0
	}
	return speed, landed
}

// Fall pulls the body by g over dt, never faster than top; one borne brakes across the pull as
// the ground holds it.
func Fall(b *Body, g geom.Vec, dt, top float64) {
	if !b.Airborne {
		down := unit(g)
		along := dot(b.Drift, down)
		across := geom.NewVec(b.Drift.X-along*down.X, b.Drift.Y-along*down.Y)
		l := math.Hypot(across.X, across.Y)
		if by := math.Hypot(g.X, g.Y) * dt; l > 0 {
			f := max(l-by, 0) / l
			b.Drift = geom.NewVec(along*down.X+across.X*f, along*down.Y+across.Y*f)
		}
	}
	b.Drift = geom.NewVec(b.Drift.X+g.X*dt, b.Drift.Y+g.Y*dt)
	if l := math.Hypot(b.Drift.X, b.Drift.Y); l > top {
		b.Drift = geom.NewVec(b.Drift.X*top/l, b.Drift.Y*top/l)
	}
}

// Leap throws a borne body against the pull g, to rise height; one in the air does not leap.
func Leap(b *Body, g geom.Vec, height float64) {
	if b.Airborne || height <= 0 {
		return
	}
	Throw(b, g, geom.Vec{}, height)
}

// Throw carries the body off at across, rising height against the pull g first.
func Throw(b *Body, g, across geom.Vec, height float64) {
	b.Drift = geom.NewVec(b.Drift.X+across.X, b.Drift.Y+across.Y)
	strength := math.Hypot(g.X, g.Y)
	if height <= 0 || strength == 0 {
		return
	}
	down := unit(g)
	if along := dot(b.Drift, down); along > 0 { // falling: the throw starts from still
		b.Drift = geom.NewVec(b.Drift.X-along*down.X, b.Drift.Y-along*down.Y)
	}
	v := math.Sqrt(2 * strength * height)
	b.Drift = geom.NewVec(b.Drift.X-down.X*v, b.Drift.Y-down.Y*v)
	b.Airborne = true
}

func dot(a, b geom.Vec) float64 { return a.X*b.X + a.Y*b.Y }

func unit(v geom.Vec) geom.Vec {
	l := math.Hypot(v.X, v.Y)
	if l == 0 {
		return v
	}
	return geom.NewVec(v.X/l, v.Y/l)
}
