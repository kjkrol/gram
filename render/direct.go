package render

import (
	"github.com/kjkrol/gram/camera"
)

// Direct is a Source that draws a part of the picture itself, with a shader of its own: in the
// passes its Compose asked for (Frame.DrawOpaque, Frame.DrawAt), where Draw is handed which. A
// Direct asking for none is not drawn.
type Direct interface {
	Source
	Draw(t Target, cam camera.Camera, u Uniforms)
}

// Pass is which part of the picture a Direct draws in: the opaque ground first, then what is seen
// through, back to front.
type Pass uint8

const (
	Opaque Pass = iota
	SeeThrough
)

// Target is where a Direct source draws: the screen, nil in a test; the depth buffer every mesh of
// the frame shares, nil in a frame without a ground; the ground's depth alone for a source that
// asked for it (Frame.ReadGround), nil in the opaque pass; and the pass.
type Target struct {
	Screen *Image
	Depth  *Depth
	Ground *Depth
	Pass   Pass
}

// Uniforms is the composer's shader's uniforms this frame, by name, for a Direct source.
type Uniforms struct{ m map[string]any }

// UniformsOf is the uniforms m, by name, each a []float32, or an *Image for a frame's image.
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
