package render

import "github.com/kjkrol/gram/render/internal/gfx"

// Image is a picture on the GPU: a texture, or a rectangle of one, drawn into and read.
type Image = gfx.Image

// Depth is a depth buffer for meshes, reversed (1 nearest).
type Depth = gfx.Depth

// Indices are a mesh's triangles, kept on the GPU once drawn.
type Indices = gfx.Indices

// IndexRange is Count of a mesh's indices from First.
type IndexRange = gfx.IndexRange

// Shader is a WGSL fragment with its uniforms, built on the materials.
type Shader = gfx.Shader

// Vertex is one corner of a triangle drawn.
type Vertex = gfx.Vertex

// Uniform is one of a shader's uniforms: Size floats, 1 to 4.
type Uniform = gfx.Uniform

// Filter is how an image drawn scaled is sampled.
type Filter = gfx.Filter

// The filters.
const (
	FilterNearest = gfx.FilterNearest
	FilterLinear  = gfx.FilterLinear
)

// GeoM is an affine transform of the plane, the identity as it comes.
type GeoM = gfx.GeoM

// DrawImageOptions is how Image.DrawImage draws.
type DrawImageOptions = gfx.DrawImageOptions

// DrawTrianglesShaderOptions is what Image.DrawTrianglesShader draws with.
type DrawTrianglesShaderOptions = gfx.DrawTrianglesShaderOptions

// DrawMeshOptions is how Image.DrawMesh draws a mesh.
type DrawMeshOptions = gfx.DrawMeshOptions

// Corners are a quad's screen corners: top-left, top-right, bottom-left, bottom-right.
type Corners = gfx.Corners

// Fade is how many pixels each side of a soft quad takes to fade out; 0 leaves a side hard.
type Fade = gfx.Fade

// SpriteID identifies a sprite in an Atlas.
type SpriteID = gfx.SpriteID

// AtlasSource supplies the sprite sheet a Frame draws from.
type AtlasSource = gfx.AtlasSource

// NewImage is a w by h image, transparent.
func NewImage(w, h int) *Image { return gfx.NewImage(w, h) }

// NewDepth is a depth buffer, fitted to the image it is drawn with.
func NewDepth() *Depth { return gfx.NewDepth() }

// NewIndices keeps the triangles of indices.
func NewIndices(indices []uint32) *Indices { return gfx.NewIndices(indices) }

// NewShaderWith is a shader on the composer's library and every material registered, with
// fragment's own uniforms and fragment after them. No material may be registered once it has
// been built.
func NewShaderWith(label string, fragment []byte, own []Uniform) *Shader {
	return gfx.NewShaderWith(label, fragment, own)
}

// NewMeshShaderWith is a mesh's shader, as NewShaderWith, whose source has a vertex stage of
// its own: vs_main and fs_main, and fs_prime for a mesh drawn Primed.
func NewMeshShaderWith(label string, source []byte, own []Uniform) *Shader {
	return gfx.NewMeshShaderWith(label, source, own)
}
