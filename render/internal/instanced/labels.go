package instanced

import (
	_ "embed"
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/render/internal/gfx"
	"github.com/kjkrol/gram/render/internal/gpu"
	"github.com/kjkrol/gram/render/internal/text"
)

//go:embed labels.wgsl
var labelsWGSL string

var labelsShader = gfx.NewMeshShaderWith("labels", []byte(labelsWGSL+"\n"),
	[]gfx.Uniform{
		{Name: "ViewSize", Size: 2},
		{Name: "ViewProj", Size: 16},
		{Name: "Wrap", Size: 4},
		{Name: "Window", Size: 4},
		{Name: "Deep", Size: 1},
		{Name: "Eye", Size: 3},
		{Name: "Bend", Size: 1},
		{Name: "Scale", Size: 1},
	}).Instanced(5)

// five vec4s a glyph or a badge: its point, the height it is seen at and its depth's; its
// top-left from the point and its size in shown pixels; its top-left on the sheet or a badge's
// radius, then a badge's shape and the width of its rim; its colour, a badge's fill; a badge's
// rim's colour
const labelFloats = 20

// BadgeShape is the outline of a badge.
type BadgeShape uint8

const (
	// BadgeDisc is a round badge.
	BadgeDisc BadgeShape = iota
	// BadgeDiamond is a square badge standing on a corner.
	BadgeDiamond
)

// shown pixels: a badge's rim, and the room round its edge for the edge's softened pixel
const (
	badgeRim    = 1.5
	badgeMargin = 1
)

// Labels is text a Direct source draws on the GPU by points of the world, a size in shown pixels
// however far the point, each glyph tested for depth as though it stood upright on its point; and
// badges, shapes with a number in, drawn alike.
type Labels struct {
	runs   []labelRun
	glyphs []float32
	pieces []gpu.Piece
	opts   DrawMeshOptions
	own    map[string][]float32
	wrap   [4]float32
}

type labelRun struct {
	font         *Font
	first, count int
}

// Wrap has the labels laid over a world w by h that wraps across where x says and down where y
// does, each where a camera's window sees it.
func (l *Labels) Wrap(w, h float32, x, y bool) {
	l.wrap = [4]float32{w, h, flag(x), flag(y)}
}

// Text adds s in font by the point (x, y) of the world seen at height z, its first line's
// top-left dx and dy shown pixels from where the point shows, standing at height deep, in c.
func (l *Labels) Text(font *Font, s string, x, y, z, deep, dx, dy float32, c color.RGBA) {
	l.pieces = text.Glyphs(font, s, l.pieces[:0])
	l.lay(font, x, y, z, deep, dx, dy, premultiplied(c))
}

// Badge adds a badge of shape width shown pixels across by the point (x, y) of the world seen at
// height z, its middle dx and dy shown pixels from where the point shows, standing at height deep,
// filled with fill within a rim in rim, and n, where it is positive, written in its middle in white
// or black, whichever stands out of fill.
func (l *Labels) Badge(n int, shape BadgeShape, x, y, z, deep, dx, dy, width float32,
	fill, rim color.RGBA) {
	font := text.NumberFont()
	l.run(font, 1)
	// on whole pixels, an even number of them, so the number's middle is the badge's
	side := 2 * f32.Ceil(width/2+badgeMargin)
	f, r := premultiplied(fill), premultiplied(rim)
	l.glyphs = append(l.glyphs, x, y, z, deep, dx-side/2, dy-side/2, side, side,
		width/2, 0, float32(shape)+1, badgeRim, f[0], f[1], f[2], f[3], r[0], r[1], r[2], r[3])
	if n <= 0 {
		return
	}
	l.pieces = text.NumberGlyphs(font, n, l.pieces[:0])
	ix, iy, iw, ih := text.Ink(l.pieces)
	ink := [4]float32{1, 1, 1, 1}
	if 0.299*float32(fill.R)+0.587*float32(fill.G)+0.114*float32(fill.B) > 0.55*255 {
		ink = [4]float32{0, 0, 0, 1}
	}
	l.lay(font, x, y, z, deep, dx+f32.Round(-ix-iw/2), dy+f32.Round(-iy-ih/2), ink)
}

// BadgeWidth is how many shown pixels across a badge of shape at least size across is to hold n
// written in its middle, nothing written where n is not positive. It may be asked on any
// goroutine.
func BadgeWidth(n int, shape BadgeShape, size float32) float32 {
	if n <= 0 {
		return size
	}
	// the corners of the number's box on the middle of the rim
	_, _, w, h := text.NumberInk(n)
	need := 2*f32.Hypot(w/2, h/2) + badgeRim
	if shape == BadgeDiamond {
		need = w + h + badgeRim*f32.Sqrt(2)
	}
	return max(size, need)
}

// premultiplied is the straight colour c in floats, multiplied by its alpha
func premultiplied(c color.RGBA) [4]float32 {
	a := float32(c.A) / 0xff
	return [4]float32{float32(c.R) / 0xff * a, float32(c.G) / 0xff * a, float32(c.B) / 0xff * a, a}
}

// lay adds the glyphs laid in l.pieces, in font, by the point (x, y) seen at height z, their
// top-left dx and dy shown pixels from where it shows, standing at height deep, in the
// premultiplied colour c
func (l *Labels) lay(font *Font, x, y, z, deep, dx, dy float32, c [4]float32) {
	if len(l.pieces) == 0 {
		return
	}
	l.run(font, len(l.pieces))
	for _, p := range l.pieces {
		l.glyphs = append(l.glyphs, x, y, z, deep, dx+p.DstX, dy+p.DstY, p.W, p.H,
			p.SrcX, p.SrcY, 0, 0, c[0], c[1], c[2], c[3], 0, 0, 0, 0)
	}
}

// run counts n more glyphs or badges in a run of font's
func (l *Labels) run(font *Font, n int) {
	if k := len(l.runs); k == 0 || l.runs[k-1].font != font {
		l.runs = append(l.runs, labelRun{font: font, first: len(l.glyphs) / labelFloats})
	}
	l.runs[len(l.runs)-1].count += n
}

// Reset drops what was gathered.
func (l *Labels) Reset() { l.runs, l.glyphs = l.runs[:0], l.glyphs[:0] }

// Draw draws the gathered text into the target's screen, a viewport cam's size, by its points as
// cam lays them under the frame's uniforms u, keeping it for another draw till Reset; against the
// target's depth, where it has one, what shows of it written. A camera that cannot say its lines
// of sight draws none.
func (l *Labels) Draw(t Target, cam camera.Camera, u Uniforms) {
	if t.Screen == nil || len(l.runs) == 0 {
		return
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	f, ok := rays.Rays()
	if !ok {
		return
	}
	w, h := cam.Viewport()
	tr, ok := camera.SceneTransform(f, w, h)
	if !ok {
		return
	}
	if l.own == nil {
		l.own, l.opts.Uniforms = map[string][]float32{}, map[string]any{}
	}
	u.Into(l.opts.Uniforms)
	l.set("ViewSize", w, h)
	l.set("ViewProj", tr.M[:]...)
	l.set("Wrap", l.wrap[:]...)
	l.set("Window", f.Origin[0], f.Origin[1], w*f.DX[0], h*f.DY[1])
	l.set("Deep", flag(t.Depth != nil))
	l.set("Eye", tr.Eye[:]...)
	l.set("Bend", tr.Bend)
	l.set("Scale", camera.ResolutionOf(cam))
	l.opts.Depth, l.opts.WriteDepth = t.Depth, t.Depth != nil
	l.opts.Vertices = 6 * (1 + int(l.wrap[2])) * (1 + int(l.wrap[3]))
	for _, r := range l.runs {
		l.opts.Images[0] = text.Sheet(r.font)
		l.opts.Instances = l.glyphs[labelFloats*r.first : labelFloats*(r.first+r.count)]
		t.Screen.DrawMesh(nil, labelsShader, &l.opts)
	}
}

// the uniform's slice is kept between frames, boxed once
func (l *Labels) set(name string, v ...float32) {
	u, ok := l.own[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		l.own[name] = u
		l.opts.Uniforms[name] = u
	}
	copy(u, v)
}
