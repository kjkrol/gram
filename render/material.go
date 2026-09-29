package render

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed library.kage
var libraryKage []byte

//go:embed compose.kage
var composeKage []byte

// MaterialID is a material registered with RegisterMaterials: what the shader works an Overlay out
// with.
type MaterialID uint8

var (
	sources       [][]byte // the materials' Kage, in the order registered
	entries       []string // by MaterialID, the function each overlay is handed to
	composeShader *ebiten.Shader
	sealed        bool // a shader has been built on the materials: no more may come
)

// RegisterMaterials adds to the composer's one shader the materials of source — Kage
// declarations, constants and functions, no package clause — one for each of entries, the
// functions an Overlay of each is handed to:
//
//	func Entry(p vec2, red float, fraction float, custom vec4) vec4
//
// where p is where the pixel lies in the world. They may call the shader's own functions (noise,
// hash, sunWay, seen, faded, blended), read its uniforms — the frame's daylight and weather — and
// call the functions of materials registered before. A plugin registers its materials as its
// package is set up, from its own .kage beside it, before the composer draws first; names are
// shared by all, so a material's own should be its own.
func RegisterMaterials(source []byte, entry ...string) []MaterialID {
	if composeShader != nil || sealed {
		panic(fmt.Sprintf("render: materials %v registered after a shader was built on them", entry))
	}
	if len(entries)+len(entry) > 1<<8 {
		panic("render: too many materials")
	}
	sources = append(sources, source)
	ids := make([]MaterialID, len(entry))
	for i, e := range entry {
		ids[i] = MaterialID(len(entries))
		entries = append(entries, e)
	}
	return ids
}

// ShaderSource is the composer's shader as it compiles: the library, every material registered,
// the function handing an overlay to its material, and the composer's Fragment.
func ShaderSource() []byte { return ShaderSourceWith(composeKage) }

// ShaderSourceWith is a shader of the composer's library — its uniforms and helpers, every
// material registered and the function handing an overlay to its material — with fragment's own
// uniforms and Fragment after them: for a source drawing itself (Direct) with the materials the
// composer has. No material may be registered once it has been asked for.
func ShaderSourceWith(fragment []byte) []byte {
	sealed = true
	var b bytes.Buffer
	b.Write(libraryKage)
	for _, src := range sources {
		b.WriteString("\n")
		b.Write(src)
	}
	b.WriteString("\n// material hands an overlay of material k to its entry.\n")
	b.WriteString("func material(k float, p vec2, red float, fraction float, custom vec4) vec4 {\n")
	for i, e := range entries {
		fmt.Fprintf(&b, "\tif k < %d.5 {\n\t\treturn %s(p, red, fraction, custom)\n\t}\n", i, e)
	}
	b.WriteString("\treturn vec4(0)\n}\n\n")
	b.Write(fragment)
	return b.Bytes()
}

// Compile compiles the composer's shader from its own part and the materials registered so far;
// the composer does it when it first draws, a game or a test may to fail early. A material
// registered after cannot be drawn.
func Compile() error {
	if composeShader != nil {
		return nil
	}
	s, err := ebiten.NewShader(ShaderSource())
	if err != nil {
		return fmt.Errorf("render: the composer's shader, with %s: %w", materialNames(), err)
	}
	composeShader = s
	return nil
}

func materialNames() string { return strings.Join(entries, ", ") }

// shader is the one program every item is drawn with, compiled at first use.
func shader() *ebiten.Shader {
	if err := Compile(); err != nil {
		panic(err)
	}
	return composeShader
}
