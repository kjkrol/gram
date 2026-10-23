package wgsl

import (
	"embed"
	"fmt"
	"slices"
	"strings"

	"github.com/kjkrol/gram/render/internal/gpu"
)

//go:embed library.wgsl noise.wgsl
var library embed.FS

// MaterialID is a material registered with RegisterMaterials, which the shader works an overlay
// out with.
type MaterialID uint8

var libraryUniforms = []gpu.Uniform{
	{Name: "Toward", Size: 3},
	{Name: "Clock", Size: 1},
	{Name: "Pixel", Size: 1},
	{Name: "Fog", Size: 3},
	{Name: "GroundLattice", Size: 4},
	{Name: "GroundSpan", Size: 4},
	{Name: "GroundShadeArea", Size: 4},
	{Name: "GroundShadeSpan", Size: 2},
	{Name: "GroundShadeMix0", Size: 4},
	{Name: "GroundShadeMix1", Size: 4},
	{Name: "GroundShadeMix2", Size: 4},
	{Name: "GroundShadeMix3", Size: 4},
}

var (
	sources  [][]byte
	uniforms = append([]gpu.Uniform(nil), libraryUniforms...)
	entries  []string // by MaterialID
	// the frame's, by their binding: the library's own first, the ground's heights and shade
	images = []string{"GroundHeights", "GroundShade", "GroundShadeWas"}
	sealed bool // a shader has been built on the materials: no more may come
)

// RegisterMaterials adds the materials of source, WGSL with the uniforms it reads (U.Name), one
// material for each of entries, the function an overlay of it is handed to. It panics once a
// shader has been built on the materials.
func RegisterMaterials(source []byte, reads []gpu.Uniform, entry ...string) []MaterialID {
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

// RegisterImage names an image every shader built on the materials may read, a texture_2d<f32>
// of that name. It panics once a shader has been built, for a name given twice and past
// gpu.FrameImages names.
func RegisterImage(name string) {
	switch {
	case sealed:
		panic(fmt.Sprintf("render: the image %s registered after a shader was built", name))
	case slices.Contains(images, name):
		panic(fmt.Sprintf("render: the image %s registered twice", name))
	case len(images) == gpu.FrameImages:
		panic(fmt.Sprintf("render: the image %s past the %d a frame holds", name, gpu.FrameImages))
	}
	images = append(images, name)
}

// Images are the frame's images registered, by their binding.
func Images() []string { return images }

// Uniforms are the library's uniforms and those the registered materials read.
func Uniforms() []gpu.Uniform { return append([]gpu.Uniform(nil), uniforms...) }

// Entries are the registered materials' entries, by MaterialID.
func Entries() []string { return entries }

// Layout is the layout of the shared uniforms, then own, sealing the materials.
func Layout(own []gpu.Uniform) *gpu.Layout {
	sealed = true
	return gpu.NewLayout(append(append([]gpu.Uniform{}, uniforms...), own...))
}

// SourceWith is fragment on the library, the frame's images and every material registered,
// sealing them.
func SourceWith(fragment []byte) string {
	sealed = true
	var b strings.Builder
	for _, n := range []string{"library.wgsl", "noise.wgsl"} {
		src, err := library.ReadFile(n)
		if err != nil {
			panic(fmt.Sprintf("render: %v", err))
		}
		b.Write(src)
		b.WriteString("\n")
	}
	for i, name := range images {
		fmt.Fprintf(&b, "@group(3) @binding(%d) var %s: texture_2d<f32>;\n", i, name)
	}
	for _, src := range sources {
		b.Write(src)
		b.WriteString("\n")
	}
	b.WriteString("// material hands an overlay of material k to its entry.\n")
	b.WriteString("fn material(k: f32, p: vec2<f32>, red: f32, fraction: f32, ")
	b.WriteString("custom: vec4<f32>) -> vec4<f32> {\n")
	for i, e := range entries {
		fmt.Fprintf(&b, "    if k < %d.5 {\n        return %s(p, red, fraction, custom);\n    }\n", i, e)
	}
	b.WriteString("    return vec4<f32>(0.0);\n}\n\n")
	b.Write(fragment)
	return b.String()
}
