package render

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/internal/light"
	"github.com/kjkrol/gram/render/internal/wgsl"
)

// Illumination is the light a thing is drawn in (light.Illumination).
type Illumination = light.Illumination

// Distant is a light from far off, the same everywhere (light.Distant).
type Distant = light.Distant

// Beam is a Distant ready to light many surfaces (light.Beam).
type Beam = light.Beam

// Umbra is where a shadow lies on the ground (light.Umbra).
type Umbra = light.Umbra

// Lamp is a lamp in the world (light.Lamp).
type Lamp = light.Lamp

// MaxLamps is how many lamps at most light a picture: the nearest its camera.
const MaxLamps = wgsl.Lamps

// Casting is how a thing casts its shadow on the ground (light.Casting).
type Casting = light.Casting

// The ways a thing casts its shadow (light.Patch…).
const (
	Patch      = light.Patch
	Silhouette = light.Silhouette
	Baked      = light.Baked
	NoShadow   = light.NoShadow
)

// Shadowing is how a thing casts its shadow and which shade it takes (light.Shadowing).
type Shadowing = light.Shadowing

// ShadowPolicy is how a thing casts its live shadow at a distance from the eye
// (light.ShadowPolicy).
type ShadowPolicy = light.ShadowPolicy

// WhiteLight is a thing drawn as it is, taking every shade: a world without a light plugin.
var WhiteLight = Illumination{Light: Light{1, 1, 1}, Takes: TakesAll}

// Even is a white Shade of v at every corner; Even(1) draws a sprite as it is.
func Even(v float32) Shade { return Lit(Light{v, v, v}) }

// Lit is a Shade of l at every corner.
func Lit(l Light) Shade { return Shade{l, l, l, l} }

// Box is the World of the world box (x0, y0)-(x1, y1).
func Box(x0, y0, x1, y1 float32) World { return World{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} }

// ProjectCorners is the four corners of the world box (x0, y0)-(x1, y1) at height z through cam.
func ProjectCorners(cam camera.Camera, x0, y0, x1, y1, z float32) Corners {
	var out Corners
	for i, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		out[i][0], out[i][1] = cam.Project(p[0], p[1], z)
	}
	return out
}
