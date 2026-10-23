package render

import (
	"embed"
	"fmt"
	"strings"

	"github.com/kjkrol/gram/internal/gpu"
	"github.com/kjkrol/gram/render/internal/wgsl"
)

//go:embed shaders/*.wgsl
var shaderFiles embed.FS

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

var composer = &Shader{label: "composer", source: func() string {
	return wgsl.SourceWith(Files(shaderFiles, "shaders/compose.wgsl"))
}, layout: func() *gpu.Layout { return wgsl.Layout(nil) }}

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

// the textures of the frame's images the uniforms u hold, by their binding, ready to be read
func frameOf(u map[string]any) (f [gpu.FrameImages]*gpu.Texture) {
	for i, name := range wgsl.Images() {
		if img, _ := u[name].(*Image); img != nil {
			f[i] = img.ready().tex
		}
	}
	return f
}

// SharedUniforms are the uniforms every shader built on the materials has: the library's and those
// the registered materials read, for a shader of one's own to keep its names apart.
func SharedUniforms() []Uniform { return wgsl.Uniforms() }

// ShaderSource is the composer's shader as it compiles.
func ShaderSource() string { return composer.Source() }

// NewShaderWith is a shader on the composer's library and every material registered, with
// fragment's own uniforms and Fragment after them. No material may be registered once it has
// been built.
func NewShaderWith(label string, fragment []byte, own []Uniform) *Shader {
	return &Shader{label: label, source: func() string { return wgsl.SourceWith(fragment) },
		layout: func() *gpu.Layout { return wgsl.Layout(own) }}
}

// Compile compiles the composer's shader now rather than at its first draw.
func Compile() error {
	if err := composer.Compile(); err != nil {
		return fmt.Errorf("render: the composer's shader, with %s: %w",
			strings.Join(wgsl.Entries(), ", "), err)
	}
	return nil
}
