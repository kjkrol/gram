// Package vec is the three-float vectors the topography's cameras and billboards work with.
package vec

import "math"

// Add is a+b.
func Add(a, b [3]float32) [3]float32 { return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }

// Sub is a−b.
func Sub(a, b [3]float32) [3]float32 { return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }

// Scale is a times s.
func Scale(a [3]float32, s float32) [3]float32 { return [3]float32{a[0] * s, a[1] * s, a[2] * s} }

// Dot is a·b.
func Dot(a, b [3]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

// Norm is a's length.
func Norm(a [3]float32) float32 { return float32(math.Sqrt(float64(Dot(a, a)))) }

// Cross is a×b.
func Cross(a, b [3]float32) [3]float32 {
	return [3]float32{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

// Unit is a the length of 1; nothing for nothing.
func Unit(a [3]float32) [3]float32 {
	if n := Norm(a); n > 0 {
		return Scale(a, 1/n)
	}
	return a
}
