package render

import (
	"github.com/kjkrol/gram/camera"
)

// Direct is a Source that draws a part of the picture itself, with a shader of its own, at its
// Tier: after every piece the frame orders before that tier and before all the rest. Draw is
// handed where to draw and the composer's uniforms this frame.
type Direct interface {
	Source
	Tier() Tier
	Draw(t Target, cam camera.Camera, u Uniforms)
}

// Target is where a Direct source draws: the screen, nil in a test, and the depth buffer every
// mesh of the frame shares.
type Target struct {
	Screen *Image
	Depth  *Depth
}

// Uniforms is the composer's shader's uniforms this frame, by name, for a Direct source.
type Uniforms struct{ m map[string]any }

// UniformsOf is the uniforms m, by name, each a []float32.
func UniformsOf(m map[string]any) Uniforms { return Uniforms{m} }

// Into puts every uniform into dst, boxed as the composer holds it.
func (u Uniforms) Into(dst map[string]any) {
	for k, v := range u.m {
		dst[k] = v
	}
}

// Get is the uniform name, nil for one the composer does not have.
func (u Uniforms) Get(name string) []float32 {
	v, _ := u.m[name].([]float32)
	return v
}
