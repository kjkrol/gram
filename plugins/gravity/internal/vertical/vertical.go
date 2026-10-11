// Package vertical is a body's fall in a world with heights: a step of the pull along the
// vertical between the surface under its feet and the slab over its head.
package vertical

import "math"

// Body is one body as a step takes it: its feet, height and the reach of its step, the drift it
// is carried at — X and Y across, Rise up — and whether it is in the air.
type Body struct {
	Feet, Height, Reach float64
	X, Y, Rise          float64
	Airborne            bool
}

// Step moves b one step of dt under a pull of g down, between floor — the surface under it —
// and ceiling, the underside of a slab over its head. A body standing follows the floor within
// its reach, the drift across braked as the ground holds it, and goes on into the air past it;
// one in the air falls, its head stopped by the ceiling, and lands on the floor: landed says so,
// at speed.
func Step(b *Body, g, floor, ceiling, dt float64) (speed float64, landed bool) {
	if !b.Airborne {
		if b.Feet-floor <= b.Reach {
			b.Feet = floor
			b.X, b.Y = brake(b.X, b.Y, g*dt)
			return 0, false
		}
		b.Airborne = true
	}
	b.Feet += b.Rise*dt - g*dt*dt/2
	b.Rise -= g * dt
	if b.Rise > 0 && b.Feet+b.Height > ceiling {
		b.Feet = ceiling - b.Height
		b.Rise = 0
	}
	if b.Rise > 0 || b.Feet > floor {
		return 0, false
	}
	speed = -b.Rise
	b.Feet = floor
	b.X, b.Y, b.Rise, b.Airborne = 0, 0, 0, false
	return speed, true
}

// Leap throws b up to rise height over where it stands under a pull of g; a body in the air
// does not leap.
func Leap(b *Body, g, height float64) {
	if b.Airborne || g <= 0 || height <= 0 {
		return
	}
	b.Rise = math.Sqrt(2 * g * height)
	b.Airborne = true
}

// Throw carries b off at x, y across, rising height up first; one thrown off without rising
// slides along the ground till the ground holds it.
func Throw(b *Body, g, x, y, height float64) {
	b.X += x
	b.Y += y
	if height > 0 && g > 0 {
		b.Rise = max(b.Rise, 0) + math.Sqrt(2*g*height)
		b.Airborne = true
	}
}

// brake takes by off the length of x, y, down to none.
func brake(x, y, by float64) (float64, float64) {
	l := math.Hypot(x, y)
	if l <= by {
		return 0, 0
	}
	f := (l - by) / l
	return x * f, y * f
}
