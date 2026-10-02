package gpu

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// Uniform is one of a program's uniforms: Size floats, 1 to 4 (f32, vec2, vec3, vec4) or 16 (a
// mat4x4, column by column).
type Uniform struct {
	Name string
	Size int
}

// Layout lays uniforms out as WGSL lays a struct of them out in a uniform buffer: their fields'
// source, where each begins and how many bytes they take in all.
type Layout struct {
	Fields  string
	offsets map[string]int
	sizes   map[string]int
	Size    int
}

// NewLayout lays out us in order.
func NewLayout(us []Uniform) *Layout {
	l := &Layout{offsets: map[string]int{}, sizes: map[string]int{}}
	var b strings.Builder
	off := 0
	for _, u := range us {
		if _, dup := l.offsets[u.Name]; dup {
			panic(fmt.Sprintf("gpu: uniform %s declared twice", u.Name))
		}
		size, align, typ := 0, 0, ""
		switch u.Size {
		case 1:
			size, align, typ = 4, 4, "f32"
		case 2:
			size, align, typ = 8, 8, "vec2<f32>"
		case 3:
			size, align, typ = 12, 16, "vec3<f32>"
		case 4:
			size, align, typ = 16, 16, "vec4<f32>"
		case 16:
			size, align, typ = 64, 16, "mat4x4<f32>"
		default:
			panic(fmt.Sprintf("gpu: uniform %s of %d floats", u.Name, u.Size))
		}
		off = (off + align - 1) / align * align
		l.offsets[u.Name], l.sizes[u.Name] = off, u.Size
		fmt.Fprintf(&b, "    %s: %s,\n", u.Name, typ)
		off += size
	}
	l.Fields = b.String()
	l.Size = (off + 15) / 16 * 16
	return l
}

// Has reports whether the layout holds the uniform name.
func (l *Layout) Has(name string) bool { _, ok := l.offsets[name]; return ok }

// Put writes v, as many floats as the uniform name holds at most, into dst, the layout's bytes;
// a name the layout lacks is ignored.
func (l *Layout) Put(dst []byte, name string, v []float32) {
	off, ok := l.offsets[name]
	if !ok {
		return
	}
	for i := 0; i < l.sizes[name] && i < len(v); i++ {
		binary.LittleEndian.PutUint32(dst[off+4*i:], math.Float32bits(v[i]))
	}
}
