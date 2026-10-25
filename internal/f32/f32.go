// Package f32 is package math for the float32 geometry gram keeps: each function is math's of
// the same name, worked out in float64 and rounded once.
package f32

import "math"

func Abs(x float32) float32 { return math.Float32frombits(math.Float32bits(x) &^ (1 << 31)) }

func Sqrt(x float32) float32 { return float32(math.Sqrt(float64(x))) }

func Hypot(x, y float32) float32 { return float32(math.Hypot(float64(x), float64(y))) }

func Floor(x float32) float32 { return float32(math.Floor(float64(x))) }

func Ceil(x float32) float32 { return float32(math.Ceil(float64(x))) }

func Round(x float32) float32 { return float32(math.Round(float64(x))) }

func Mod(x, y float32) float32 { return float32(math.Mod(float64(x), float64(y))) }

func Sin(x float32) float32 { return float32(math.Sin(float64(x))) }

func Cos(x float32) float32 { return float32(math.Cos(float64(x))) }

func Tan(x float32) float32 { return float32(math.Tan(float64(x))) }

func Sincos(x float32) (sin, cos float32) {
	s, c := math.Sincos(float64(x))
	return float32(s), float32(c)
}

func Atan2(y, x float32) float32 { return float32(math.Atan2(float64(y), float64(x))) }

func Exp(x float32) float32 { return float32(math.Exp(float64(x))) }

func Remainder(x, y float32) float32 { return float32(math.Remainder(float64(x), float64(y))) }

func Copysign(f, sign float32) float32 { return float32(math.Copysign(float64(f), float64(sign))) }

func Inf(sign int) float32 { return float32(math.Inf(sign)) }

func IsInf(f float32, sign int) bool { return math.IsInf(float64(f), sign) }

// Eps is how far apart two float32 values around scale may lie and still be one: some eight
// float32 steps at scale, at least 1e-6.
func Eps(scale float32) float32 { return 1e-6 * max(Abs(scale), 1) }
