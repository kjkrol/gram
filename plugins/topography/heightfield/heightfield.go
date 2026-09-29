// Package heightfield draws a relief on the GPU from its heightmap: one shader traces every pixel's
// line of sight over the ground (heightfield.kage), walking the lattice cell by cell and meeting
// each cell's two triangles exactly, in place of the board's tiles — a render.Direct source at
// the Ground tier, the topography's other way of drawing the ground.
//
// The shader is built on the composer's library (render.ShaderSourceWith): it reads the sun, the
// air and the clock the frame set, and draws the water with the topography's materials. The
// Renderer keeps the lattice in one image of four quadrants — heights (16 bits a corner), the way
// the ground faces, each cell's highest corner, the way to the shore — written anew as the ground
// or the coast changes, and reads the ground's look from whoever draws the board (Surface): the
// board painted flat, its water in layers (WaterLayers), the shores and the grid. What stands on
// the ground is drawn by the world's renderer as before; Hides says what the ground hides from
// the eye, so a billboard behind a hill is left out.
package heightfield

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"

	_ "embed"
)

//go:embed heightfield.kage
var kage []byte

// Ground is the relief as the renderer reads it: its heights as a lattice over a square grid —
// cols by rows corners a cell apart, row by row, false off a square grid — the height at any
// point, and a count of its changes. topography.Relief is one.
type Ground interface {
	Lattice() (cols, rows int, cell float32, heights []float32, ok bool)
	At(p geom.Vec) float64
	Version() uint64
}

// Surface is what the ground looks like beyond its heights, as whoever draws the board has it,
// brought up to date as it is asked for.
type Surface interface {
	Surface() Painted
}

// Painted is the ground's look: the board painted flat (Albedo, Px pixels a cell from the
// lattice's top-left; nil, plain grey) and its water (Water, WaterPx pixels a cell, in quadrants
// of cols·WaterPx by rows·WaterPx: running water's flow and coverage top-left, the running and
// the still water's shine and coverage top-right, the glint where a way turns into water
// bottom-left — see WaterLayers; nil, none); the way to the shore from every lattice corner
// (Shores, row by row; Reach the farthest a shore is seen from; Coast counts their changes); and
// the grid, drawn where a cell spans Grid screen pixels or more, 0 for none.
type Painted struct {
	Albedo, Water *ebiten.Image
	Px, WaterPx   int
	Shores        []Shore
	Reach         float32
	Coast         uint64
	Grid          float32
}

// Shore is the way from a lattice corner to the nearest shore, of length 1 or none, how far it is
// and how near: 1 on it down to 0 at Painted.Reach.
type Shore struct{ X, Y, Dist, Near float32 }

// WaterLayers is how the water is painted, each value times the coverage W of its layer, the
// flows v as v/(2·FlowSpan)+½: top-left (vx, vy, Wrun), top-right (run shine·Wrun, sea shine·Wsea,
// Wsea), bottom-left (way glint·Wglint, Wglint, 0); what covers the water paints them black.
const WaterLayers = 3

// FlowSpan is the fastest water the water's flow holds, world units a second: faster runs white
// anyway.
const FlowSpan = 64

// Sky is the air over the ground, for how far one sees through it.
type Sky interface {
	Air() air.Weather
}

// Config is how the ground is traced: whether it casts its shadows (Shadows), the world's Scale,
// for how far one sees through the air, and Downscale, how many times smaller than the viewport
// the picture is traced and then scaled up — 2 unless set, a quarter of the pixels; 1 every
// pixel.
type Config struct {
	Shadows   bool
	Scale     world.Scale
	Downscale int
}

// Renderer draws the ground from its heightmap: a render.Direct source at the Ground tier.
type Renderer struct {
	ground  Ground
	surface Surface
	sky     Sky
	cfg     Config

	// lattice holds by lattice corner, in quadrants: heights, normals, cells' tops, shores
	lattice   *ebiten.Image
	heightsAt uint64 // one more than the ground's version the heights hold; 0 none
	coastAt   uint64 // one more than the coast's version the shores hold; 0 none
	low, span float32
	buf       []byte
	plain     *ebiten.Image // the ground's colour without an albedo: grey
	painted   Painted       // the surface as Draw last read it

	shader   *ebiten.Shader
	opts     ebiten.DrawTrianglesShaderOptions
	uniforms map[string][]float32 // the renderer's own, and boxed once for the options
	boxed    map[string]any
	verts    []ebiten.Vertex
	indices  []uint16
	hidden   bool
	off      *ebiten.Image // the picture traced smaller than the viewport, scaled up onto it
	scaling  ebiten.DrawImageOptions
}

var _ render.Direct = (*Renderer)(nil)

// New is a renderer of ground looking as surface says under sky, as cfg says; it draws nothing
// while Hidden.
func New(ground Ground, surface Surface, sky Sky, cfg Config) *Renderer {
	if cfg.Downscale <= 0 {
		cfg.Downscale = 2
	}
	r := &Renderer{ground: ground, surface: surface, sky: sky, cfg: cfg, uniforms: map[string][]float32{}, boxed: map[string]any{}}
	r.opts.Uniforms = map[string]any{}
	r.indices = []uint16{0, 1, 2, 1, 2, 3}
	r.verts = make([]ebiten.Vertex, 4)
	r.scaling.Filter = ebiten.FilterLinear
	return r
}

// Source is the ground's shader as it compiles: the composer's library and materials — the sun,
// the air, the topography's water — with the ground's own tracing after them.
func Source() []byte { return render.ShaderSourceWith(kage) }

func (r *Renderer) Init(*goke.SysInit) {}

// Compose hands the frame nothing: the ground is drawn Direct.
func (r *Renderer) Compose(*render.Frame, camera.Camera) {}

// Tier is where the ground comes in the picture: render.Ground, after the sky.
func (r *Renderer) Tier() render.Tier { return render.Ground }

// Hide has the renderer draw nothing, or draw again: the tiles take its place while hidden.
func (r *Renderer) Hide(hidden bool) { r.hidden = hidden }

// Hidden reports whether the renderer draws nothing.
func (r *Renderer) Hidden() bool { return r.hidden }

// Draw traces the ground over the whole of screen through cam — a camera with Rays — under the
// frame's uniforms u; nothing through another camera, while Hidden, or off a square grid.
func (r *Renderer) Draw(screen *ebiten.Image, cam camera.Camera, u render.Uniforms) {
	if screen == nil || r.hidden || !r.prepare(cam, u) {
		return
	}
	if r.shader == nil {
		s, err := ebiten.NewShader(Source())
		if err != nil {
			panic("heightfield: " + err.Error())
		}
		r.shader = s
	}
	w, h := cam.Viewport()
	albedo := r.painted.Albedo
	if albedo == nil {
		if r.plain == nil {
			r.plain = ebiten.NewImage(1, 1)
			r.plain.Fill(color.RGBA{128, 128, 128, 255})
		}
		albedo = r.plain
	}
	r.opts.Images[0], r.opts.Images[1], r.opts.Images[2], r.opts.Images[3] = r.lattice, albedo, r.painted.Water, nil
	// traced over the viewport itself, or over a smaller picture whose pixels look along the
	// viewport's lines of sight, scaled up onto it
	dst, k := screen, float32(r.cfg.Downscale)
	if k > 1 {
		ow, oh := max(int(w/k), 1), max(int(h/k), 1)
		if r.off == nil || r.off.Bounds().Dx() != ow || r.off.Bounds().Dy() != oh {
			r.off = ebiten.NewImage(ow, oh)
		}
		r.off.Clear()
		dst, k = r.off, w/float32(ow)
	} else {
		k = 1
	}
	dw, dh := float32(dst.Bounds().Dx()), float32(dst.Bounds().Dy())
	for i, p := range [4][2]float32{{0, 0}, {dw, 0}, {0, dh}, {dw, dh}} {
		r.verts[i] = ebiten.Vertex{DstX: p[0], DstY: p[1], ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: p[0] * k, Custom1: p[1] * k}
	}
	dst.DrawTrianglesShader(r.verts, r.indices, r.shader, &r.opts)
	if dst != screen {
		r.scaling.GeoM.Reset()
		r.scaling.GeoM.Scale(float64(w/dw), float64(h/dh))
		screen.DrawImage(r.off, &r.scaling)
	}
}

// prepare readies a draw through cam under the frame's uniforms u: the lattice and the surface
// brought up to date, the frame's uniforms and the renderer's own over them; false where there is
// nothing to draw.
func (r *Renderer) prepare(cam camera.Camera, u render.Uniforms) bool {
	rays, ok := cam.(camera.Rays)
	if !ok {
		return false
	}
	field, ok := rays.Rays()
	if !ok || !r.refresh() {
		return false
	}
	u.Into(r.opts.Uniforms)
	r.setUniforms(field, cam, u)
	return true
}

// refresh brings the lattice up to date with the ground and the surface's shores, and reads the
// surface; false where there is no lattice to draw, or one too large for the image.
func (r *Renderer) refresh() bool {
	cols, rows, cell, heights, ok := r.ground.Lattice()
	if !ok || cols < 2 || rows < 2 || 2*cols > maxLattice || 2*rows > maxLattice {
		return false
	}
	if r.lattice == nil || r.lattice.Bounds().Dx() != 2*cols || r.lattice.Bounds().Dy() != 2*rows {
		r.lattice = ebiten.NewImage(2*cols, 2*rows)
		r.heightsAt, r.coastAt = 0, 0
	}
	if v := r.ground.Version() + 1; r.heightsAt != v {
		r.heightsAt = v
		r.low, r.span, r.buf = encode(heights, r.buf[:0])
		r.write(0, 0, cols, rows)
		r.buf = normals(cols, rows, cell, heights, r.buf[:0])
		r.write(cols, 0, cols, rows)
		r.buf = cellTops(cols, rows, heights, r.low, r.span, r.buf[:0])
		r.write(0, rows, cols, rows)
	}
	r.painted = r.surface.Surface()
	if v := r.painted.Coast + 1; r.coastAt != v {
		r.coastAt = v
		r.buf = shores(cols, rows, r.painted.Shores, r.painted.Reach, r.buf[:0])
		r.write(cols, rows, cols, rows)
	}
	return true
}

// maxLattice is the widest the lattice's image may be: two quadrants of corners a side.
const maxLattice = 4096

// write writes the buffer into the lattice's quadrant w by h from (x, y).
func (r *Renderer) write(x, y, w, h int) {
	r.lattice.SubImage(image.Rect(x, y, x+w, y+h)).(*ebiten.Image).WritePixels(r.buf)
}

// shores is the way to the shore from every lattice corner as the lattice holds it: (dir+1)/2 in
// red and green, the distance over reach in blue; open water at every corner without one; appended
// to dst.
func shores(cols, rows int, from []Shore, reach float32, dst []byte) []byte {
	unit := func(v float32) byte { return byte(min(max(v, 0), 1)*255 + 0.5) }
	for i := range cols * rows {
		s := Shore{Dist: reach}
		if i < len(from) {
			s = from[i]
		}
		far := float32(1)
		if reach > 0 {
			far = s.Dist / reach
		}
		dst = append(dst, unit((s.X+1)/2), unit((s.Y+1)/2), unit(far), 255)
	}
	return dst
}

// cellTops is every cell's highest corner, at the cell's top-left corner of a lattice-sized
// image, in 16 bits from low over span as encode has them, rounded up so nothing of the cell
// stands over it; appended to dst.
func cellTops(cols, rows int, heights []float32, low, span float32, dst []byte) []byte {
	for y := range rows {
		for x := range cols {
			if x == cols-1 || y == rows-1 {
				dst = append(dst, 0, 0, 0, 255)
				continue
			}
			i := y*cols + x
			h := max(heights[i], heights[i+1], heights[i+cols], heights[i+cols+1])
			v := uint32(min(math.Ceil(float64(min(max((h-low)/span, 0), 1)*65535)), 65535))
			dst = append(dst, byte(v>>8), byte(v&255), 0, 255)
		}
	}
	return dst
}

// normals is the way the ground faces at every corner of the lattice — from its rise between the
// corners either side, one-sided at the edge, as the tiles' light has it — as an image holds it:
// (n+1)/2 in red, green and blue, appended to dst.
func normals(cols, rows int, cell float32, heights []float32, dst []byte) []byte {
	at := func(x, y int) float32 { return heights[y*cols+x] }
	rise := func(x, y, dx, dy int) float32 {
		x0, y0, x1, y1 := x-dx, y-dy, x+dx, y+dy
		steps := float32(2)
		if x0 < 0 || y0 < 0 {
			x0, y0, steps = x, y, 1
		}
		if x1 >= cols || y1 >= rows {
			x1, y1, steps = x, y, steps-1
		}
		if steps <= 0 {
			return 0
		}
		return (at(x1, y1) - at(x0, y0)) / (steps * cell)
	}
	for y := range rows {
		for x := range cols {
			n := [3]float32{-rise(x, y, 1, 0), -rise(x, y, 0, 1), 1}
			l := float32(math.Sqrt(float64(n[0]*n[0] + n[1]*n[1] + 1)))
			for k := range n {
				dst = append(dst, byte((n[k]/l+1)/2*255+0.5))
			}
			dst = append(dst, 255)
		}
	}
	return dst
}

// decodeNormal is the way a corner faces as its bytes hold it.
func decodeNormal(r, g, b byte) [3]float32 {
	return [3]float32{float32(r)/255*2 - 1, float32(g)/255*2 - 1, float32(b)/255*2 - 1}
}

// encode is heights as the heightmap holds them — 16 bits a corner, red the high byte and green
// the low, of the way from the lowest (low) to the highest — appended to dst.
func encode(heights []float32, dst []byte) (low, span float32, out []byte) {
	low, high := float32(math.Inf(1)), float32(math.Inf(-1))
	for _, h := range heights {
		low, high = min(low, h), max(high, h)
	}
	if len(heights) == 0 {
		low, high = 0, 0
	}
	span = max(high-low, 1)
	for _, h := range heights {
		v := uint32(min(max((h-low)/span, 0), 1)*65535 + 0.5)
		dst = append(dst, byte(v>>8), byte(v&255), 0, 255)
	}
	return low, span, dst
}

// decode is the height a corner encoded as (hi, lo) stands for, as the shader reads it.
func decode(hi, lo byte, low, span float32) float32 {
	return low + float32(uint32(hi)<<8|uint32(lo))/65535*span
}

// setUniforms hands the shader the lines of sight, the lattice, the surface, the grid and how far
// one sees, over the frame's u: the sun, the clouds, the clock, a world unit a screen pixel spans
// and the rest come from the frame — the waves as fine as on the tiles, blurred by the tracing.
func (r *Renderer) setUniforms(f camera.RayField, cam camera.Camera, u render.Uniforms) {
	cols, rows, cell, _, _ := r.ground.Lattice()
	r.set("Origin", f.Origin[:]...)
	r.set("DX", f.DX[:]...)
	r.set("DY", f.DY[:]...)
	r.set("Dir", f.Dir[:]...)
	r.set("DDX", f.DDX[:]...)
	r.set("DDY", f.DDY[:]...)
	r.set("Bend", f.Bend)
	r.set("Cell", cell)
	r.set("Corners", float32(cols), float32(rows))
	r.set("Low", r.low)
	r.set("Span", r.span)
	r.set("ShoreReach", max(r.painted.Reach, 1e-3))
	albedo := r.painted.Albedo
	px, size := float32(r.painted.Px), [2]float32{1, 1}
	if albedo != nil {
		size = [2]float32{float32(albedo.Bounds().Dx()), float32(albedo.Bounds().Dy())}
	} else {
		px = 1
	}
	r.set("Px", px)
	r.set("AlbedoSize", size[:]...)
	wpx := float32(0)
	if r.painted.Water != nil {
		wpx = float32(r.painted.WaterPx)
	}
	r.set("WaterPx", wpx)
	r.set("FlowSpan", FlowSpan)
	shadows := float32(0)
	if r.cfg.Shadows {
		shadows = 1
	}
	r.set("Shadows", shadows)
	r.set("GridFrom", r.painted.Grid)
	r.set("Trace", float32(r.cfg.Downscale))
	visibility := float32(0)
	if _, eyed := cam.(camera.Eyed); eyed && r.sky != nil {
		visibility = float32(air.Visibility(r.cfg.Scale, r.sky.Air()))
	}
	r.set("Visibility", visibility)
}

// set hands the shader the uniform name as v, in a slice kept between frames and boxed once, over
// whatever the frame's uniforms put there.
func (r *Renderer) set(name string, v ...float32) {
	u, ok := r.uniforms[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		r.uniforms[name] = u
		r.boxed[name] = u
	}
	copy(u, v)
	r.opts.Uniforms[name] = r.boxed[name]
}

// Hides reports whether the ground hides the world point (x, y, z) from cam's eye: the line of
// sight to it, walked from where it starts every half cell, meets ground standing over it before
// the point — its own cell's ground aside. Nothing is hidden while the renderer is Hidden, the
// tiles' painter's order hiding what they hide, or through a camera without Rays.
func (r *Renderer) Hides(cam camera.Camera, x, y, z float32) bool {
	if r.hidden {
		return false
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return false
	}
	field, ok := rays.Rays()
	if !ok {
		return false
	}
	_, _, cell, _, ok := r.ground.Lattice()
	if !ok || cell <= 0 {
		return false
	}
	sx, sy := cam.Project(x, y, z)
	o, d := field.At(sx, sy)
	n := float32(math.Sqrt(float64(d[0]*d[0] + d[1]*d[1] + d[2]*d[2])))
	if n == 0 {
		return false
	}
	d = [3]float32{d[0] / n, d[1] / n, d[2] / n}
	total := (x-o[0])*d[0] + (y-o[1])*d[1] + (z-o[2])*d[2]
	step := cell / 2
	for t := step; t < total-step; t += step {
		px, py, pz := o[0]+d[0]*t, o[1]+d[1]*t, o[2]+d[2]*t
		dx, dy := px-o[0], py-o[1]
		if float64(pz) < r.ground.At(geom.NewVec(float64(px), float64(py)))-float64(field.Bend*(dx*dx+dy*dy)) {
			return true
		}
	}
	return false
}
