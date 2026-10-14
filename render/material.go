package render

import (
	"embed"
	"fmt"
	"strings"

	"github.com/kjkrol/gram/render/gpu"
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
type MaterialID uint8

// Look is the ways a look is painted: drawn once into the sheet (a SpriteDrawer), worked out
// per pixel every frame (a MaterialID), frames along an effect's course (Phases) or frames
// round and round by the clock (Loops).
type Look interface {
	~func(dst *Canvas, size int) | MaterialID | Phases | Loops
}

var libraryUniforms = []Uniform{{Name: "Toward", Size: 3}, {Name: "Clock", Size: 1}, {Name: "Pixel", Size: 1}, {Name: "Fog", Size: 3}}

var (
	sources  [][]byte
	uniforms = append([]Uniform(nil), libraryUniforms...)
	entries  []string // by MaterialID
	sealed   bool     // a shader has been built on the materials: no more may come
	composer = &Shader{label: "composer", source: func() string { return shaderSourceWith(Files(shaderFiles, "shaders/compose.wgsl")) }, layout: composerLayout}
)

// RegisterMaterials adds to the composer's shader the materials of source, WGSL with the uniforms
// it reads (U.Name), one material for each of entries, the function an Overlay of it is handed to:
//
//	fn Entry(p: vec2<f32>, red: f32, fraction: f32, custom: vec4<f32>) -> vec4<f32>
//
// It panics once a shader has been built on the materials.
func RegisterMaterials(source []byte, reads []Uniform, entry ...string) []MaterialID {
	if sealed {
		panic(fmt.Sprintf("render: materials %v registered after a shader was built on them", entry))
	}
	if len(entries)+len(entry) > 1<<8 {
		panic("render: too many materials")
	}
	sources = append(sources, source)
	uniforms = append(uniforms, reads...)
	ids := make([]MaterialID, len(entry))
	for i, e := range entry {
		ids[i] = MaterialID(len(entries))
		entries = append(entries, e)
	}
	return ids
}

// ShaderSource is the composer's shader as it compiles.
func ShaderSource() string { return composer.Source() }

// NewShaderWith is a shader on the composer's library and every material registered, with
// fragment's own uniforms and Fragment after them. No material may be registered once it has
// been built.
func NewShaderWith(label string, fragment []byte, own []Uniform) *Shader {
	return &Shader{label: label, source: func() string { return shaderSourceWith(fragment) }, layout: func() *gpu.Layout {
		sealed = true
		return gpu.NewLayout(append(append([]Uniform{}, uniforms...), own...))
	}}
}

func composerLayout() *gpu.Layout {
	sealed = true
	return gpu.NewLayout(uniforms)
}

func shaderSourceWith(fragment []byte) string {
	sealed = true
	var b strings.Builder
	b.Write(Files(shaderFiles, "shaders/library.wgsl", "shaders/noise.wgsl"))
	for _, src := range sources {
		b.Write(src)
		b.WriteString("\n")
	}
	b.WriteString("// material hands an overlay of material k to its entry.\n")
	b.WriteString("fn material(k: f32, p: vec2<f32>, red: f32, fraction: f32, custom: vec4<f32>) -> vec4<f32> {\n")
	for i, e := range entries {
		fmt.Fprintf(&b, "    if k < %d.5 {\n        return %s(p, red, fraction, custom);\n    }\n", i, e)
	}
	b.WriteString("    return vec4<f32>(0.0);\n}\n\n")
	b.Write(fragment)
	return b.String()
}

// Compile compiles the composer's shader now rather than at its first draw.
func Compile() error {
	if err := composer.Compile(); err != nil {
		return fmt.Errorf("render: the composer's shader, with %s: %w", strings.Join(entries, ", "), err)
	}
	return nil
}
