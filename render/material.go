package render

import (
	"embed"
	"fmt"
	"strings"

	"github.com/kjkrol/gram/render/internal/compose"
	"github.com/kjkrol/gram/render/internal/wgsl"
)

// Files joins the named files of fs, in order, into one WGSL source.
func Files(fs embed.FS, names ...string) []byte {
	var b strings.Builder
	for _, n := range names {
		src, err := fs.ReadFile(n)
		if err != nil {
			panic(fmt.Sprintf("render: %v", err))
		}
		b.Write(src)
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// MaterialID is a material registered with RegisterMaterials, which the shader works an Overlay
// out with.
type MaterialID = wgsl.MaterialID

// Look is the ways a look is painted: drawn once into the sheet (a SpriteDrawer), worked out
// per pixel every frame (a MaterialID), frames along an effect's course (Phases) or frames
// round and round by the clock (Loops).
type Look interface {
	~func(dst *Canvas, size int) | MaterialID | Phases | Loops
}

// RegisterMaterials adds to the composer's shader the materials of source, WGSL with the uniforms
// it reads (U.Name), one material for each of entries, the function an Overlay of it is handed to:
//
//	fn Entry(p: vec2<f32>, red: f32, fraction: f32, custom: vec4<f32>) -> vec4<f32>
//
// It panics once a shader has been built on the materials.
func RegisterMaterials(source []byte, reads []Uniform, entry ...string) []MaterialID {
	return wgsl.RegisterMaterials(source, reads, entry...)
}

// RegisterImage names an image every shader built on the materials may read, a texture_2d<f32> of
// that name in WGSL, which a frame hands them (Frame.Image). It panics once a shader has been
// built, for a name given twice and past gpu.FrameImages names.
func RegisterImage(name string) { wgsl.RegisterImage(name) }

// SharedUniforms are the uniforms every shader built on the materials has: the library's and those
// the registered materials read, for a shader of one's own to keep its names apart.
func SharedUniforms() []Uniform { return wgsl.Uniforms() }

// ShaderSource is the composer's shader as it compiles.
func ShaderSource() string { return compose.ShaderSource() }

// Compile compiles the composer's shader now rather than at its first draw.
func Compile() error { return compose.Compile() }
