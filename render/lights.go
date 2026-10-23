package render

import "github.com/kjkrol/gram/camera"

// Illumination is the light a thing is drawn in: Light on a surface facing up, the sun's and the
// lamps', Lamps the lamps' alone, and which shade it Takes.
type Illumination struct {
	Light, Lamps Light
	// Direct is the share of Light that is the distant light's, 0 to 1: what a shadow takes away.
	Direct float32
	Takes  Taking
}

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
