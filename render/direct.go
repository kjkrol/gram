package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
)

// Direct is a Source that draws a part of the picture itself, with a shader of its own, straight on
// the screen — a heightfield traced per pixel — at its Tier: after every piece the frame orders
// before that tier and before all the rest. Compose may hand the frame pieces too, or nothing.
// Draw is handed the screen a test leaves nil — it draws nothing then — and the shader's uniforms
// as the composer has them this frame: its own and every one a source set (Frame.Uniform), for a
// shader built on the composer's library (ShaderSourceWith).
type Direct interface {
	Source
	Tier() Tier
	Draw(screen *ebiten.Image, cam camera.Camera, u Uniforms)
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
