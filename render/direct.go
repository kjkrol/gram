package render

import (
	"github.com/kjkrol/gram/camera"
)

// Direct is a Source that draws a part of the picture itself, with a shader of its own, straight on
// the screen — the ground as a mesh, the sky — at its Tier: after every piece the frame orders
// before that tier and before all the rest. Compose may hand the frame pieces too, or nothing.
// Draw is handed where to draw (Target) and the shader's uniforms as the composer has them this
// frame: its own and every one a source set (Frame.Uniform), for a shader built on the composer's
// library (NewShaderWith, NewMeshShaderWith).
type Direct interface {
	Source
	Tier() Tier
	Draw(t Target, cam camera.Camera, u Uniforms)
}

// Target is where a Direct source draws: the screen — nil in a test, when it draws nothing — and
// the depth buffer every mesh of the frame shares, cleared as the frame begins.
type Target struct {
	Screen *Image
	Depth  *Depth
}

// Uniforms is the composer's shader's uniforms this frame, by name, for a Direct source: read, or
// put into the uniforms of its own draw.
type Uniforms struct{ m map[string]any }

// UniformsOf is the uniforms m, by name, each a []float32: for a test, or a Direct source drawn
// without a composer.
func UniformsOf(m map[string]any) Uniforms { return Uniforms{m} }

// Into puts every uniform into dst, as boxed as the composer holds it: nothing is allocated.
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
