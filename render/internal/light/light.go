// Package light is how render lights a scene: the distant light and the lamps, the light on what
// is drawn, and the shadows cast.
package light

import "math"

// Light is a colour of light, red, green and blue, a colour drawn in it is scaled by; 1 each is
// white.
type Light [3]float32

// Taking is which shade a drawn thing takes, bits of it: the relief's shadow, the baked blocks',
// the clouds', and which light, the lamps'. One taking none shines as it is drawn.
type Taking uint8

const (
	TakesRelief Taking = 1 << iota
	TakesBlocks
	TakesClouds
	TakesLamps
	// Shines takes nothing: the thing is drawn as it is.
	Shines Taking = 1 << 7
	// TakesAll takes every shade and the lamps' light, as a zero Taking does.
	TakesAll = TakesRelief | TakesBlocks | TakesClouds | TakesLamps
)

// Illumination is the light a thing is drawn in: Light on a surface facing up, the sun's and the
// lamps', Lamps the lamps' alone, and which shade it Takes.
type Illumination struct {
	Light, Lamps Light
	// Direct is the share of Light that is the distant light's, 0 to 1: what a shadow takes away.
	Direct float32
	Takes  Taking
}

// Uniformer takes the uniforms of a frame by name.
type Uniformer interface {
	Uniform(name string, v ...float32)
}

func white(l Light) Light {
	if l == (Light{}) {
		return Light{1, 1, 1}
	}
	return l
}

func sqrt32(v float32) float32 { return float32(math.Sqrt(float64(v))) }
