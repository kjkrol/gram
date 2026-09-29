// Package heightfield draws a relief on the GPU from its heightmap: one shader traces every pixel's
// line of sight over the ground (heightfield.kage) in place of the board's tiles — a
// render.Direct source at the Ground tier, the topography's other way of drawing the ground.
//
// The Renderer keeps the ground's lattice of heights in an image, 16 bits a corner, and a colour a
// cell in another, both written anew when the ground or the board has changed; every frame it
// hands the shader the camera's lines of sight (camera.Rays), the sun and the air, and draws one
// quad over the viewport. What stands on the ground is drawn by the world's renderer as before;
// Hides says what the ground hides from the eye, so a billboard behind a hill is left out.
package heightfield

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
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

// Colours is what the ground is coloured by: a colour a cell of the square grid, and a count of
// the changes.
type Colours interface {
	Size() (cols, rows int)
	Colour(x, y int) color.RGBA
	Version() uint64
}

// Sky is the light and the air over the ground.
type Sky interface {
	Sun() sky.Sun
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
	colours Colours
	sky     Sky
	cfg     Config

	heights, albedo *ebiten.Image
	heightsAt       uint64 // one more than the ground's version the heightmap holds; 0 none
	coloursAt       uint64
	low, span       float32
	buf             []byte

	shader   *ebiten.Shader
	opts     ebiten.DrawTrianglesShaderOptions
	uniforms map[string][]float32
	verts    []ebiten.Vertex
	indices  []uint16
	hidden   bool
	off      *ebiten.Image // the picture traced smaller than the viewport, scaled up onto it
	scaling  ebiten.DrawImageOptions
}

var _ render.Direct = (*Renderer)(nil)

// New is a renderer of ground coloured by colours under sky, as cfg says; it draws nothing while
// Hidden.
func New(ground Ground, colours Colours, sky Sky, cfg Config) *Renderer {
	if cfg.Downscale <= 0 {
		cfg.Downscale = 2
	}
	r := &Renderer{ground: ground, colours: colours, sky: sky, cfg: cfg, uniforms: map[string][]float32{}}
	r.opts.Uniforms = map[string]any{}
	r.indices = []uint16{0, 1, 2, 1, 2, 3}
	r.verts = make([]ebiten.Vertex, 4)
	r.scaling.Filter = ebiten.FilterLinear
	return r
}

func (r *Renderer) Init(*goke.SysInit) {}

// Compose hands the frame nothing: the ground is drawn Direct.
func (r *Renderer) Compose(*render.Frame, camera.Camera) {}

// Tier is where the ground comes in the picture: render.Ground, after the sky.
func (r *Renderer) Tier() render.Tier { return render.Ground }

// Hide has the renderer draw nothing, or draw again: the tiles take its place while hidden.
func (r *Renderer) Hide(hidden bool) { r.hidden = hidden }

// Hidden reports whether the renderer draws nothing.
func (r *Renderer) Hidden() bool { return r.hidden }

// Draw traces the ground over the whole of screen through cam — a camera with Rays; nothing through
// another, while Hidden, or off a square grid.
func (r *Renderer) Draw(screen *ebiten.Image, cam camera.Camera) {
	if screen == nil || r.hidden {
		return
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	field, ok := rays.Rays()
	if !ok || !r.refresh() {
		return
	}
	if r.shader == nil {
		s, err := ebiten.NewShader(kage)
		if err != nil {
			panic("heightfield: " + err.Error())
		}
		r.shader = s
	}
	r.setUniforms(field, cam)
	w, h := cam.Viewport()
	r.opts.Images[0], r.opts.Images[1] = r.heights, r.albedo
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

// refresh brings the heightmap and the colours up to date with the ground and the board; false
// where there is no lattice to draw.
func (r *Renderer) refresh() bool {
	cols, rows, _, heights, ok := r.ground.Lattice()
	if !ok || cols < 2 || rows < 2 {
		return false
	}
	if v := r.ground.Version() + 1; r.heightsAt != v || r.heights == nil {
		r.heightsAt = v
		if r.heights == nil || r.heights.Bounds().Dx() != cols || r.heights.Bounds().Dy() != rows {
			r.heights = ebiten.NewImage(cols, rows)
		}
		r.low, r.span, r.buf = encode(heights, r.buf[:0])
		r.heights.WritePixels(r.buf)
	}
	// the colours' image is the heightmap's size, a cell's colour at its top-left corner: the
	// shader reads every image at a position in the first one's texture, within its region
	if v := r.colours.Version() + 1; r.coloursAt != v || r.albedo == nil || r.albedo.Bounds().Dx() != cols || r.albedo.Bounds().Dy() != rows {
		r.coloursAt = v
		if r.albedo == nil || r.albedo.Bounds().Dx() != cols || r.albedo.Bounds().Dy() != rows {
			r.albedo = ebiten.NewImage(cols, rows)
		}
		cw, ch := r.colours.Size()
		r.buf = r.buf[:0]
		for y := range rows {
			for x := range cols {
				var c color.RGBA
				if x < cw && y < ch {
					c = r.colours.Colour(x, y)
				}
				r.buf = append(r.buf, c.R, c.G, c.B, c.A)
			}
		}
		r.albedo.WritePixels(r.buf)
	}
	return true
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

// setUniforms hands the shader the lines of sight, the lattice, the sun and the air.
func (r *Renderer) setUniforms(f camera.RayField, cam camera.Camera) {
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
	sun, weather := r.sky.Sun(), r.sky.Air()
	dir := sun.Dir
	if n := float32(math.Sqrt(float64(dir[0]*dir[0] + dir[1]*dir[1] + dir[2]*dir[2]))); n > 0 {
		dir = [3]float32{dir[0] / n, dir[1] / n, dir[2] / n}
	}
	r.set("Sun", dir[:]...)
	r.set("SunStrength", sun.Strength)
	c, s := white(sun.Color), sun.SkyLight()
	r.set("SunColor", c[0], c[1], c[2])
	r.set("Ambience", sun.Ambient*s[0], sun.Ambient*s[1], sun.Ambient*s[2])
	shadows := float32(0)
	if r.cfg.Shadows {
		shadows = 1
	}
	r.set("Shadows", shadows)
	fog := air.Overcast(s, weather.Clouds)
	r.set("Fog", fog[0], fog[1], fog[2])
	visibility := float32(0)
	if _, eyed := cam.(camera.Eyed); eyed {
		visibility = float32(air.Visibility(r.cfg.Scale, weather))
	}
	r.set("Visibility", visibility)
}

// set hands the shader the uniform name as v, in a slice kept between frames.
func (r *Renderer) set(name string, v ...float32) {
	u, ok := r.uniforms[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		r.uniforms[name] = u
		r.opts.Uniforms[name] = u
	}
	copy(u, v)
}

// white is l, or white for the zero light.
func white(l render.Light) render.Light {
	if l == (render.Light{}) {
		return render.Light{1, 1, 1}
	}
	return l
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
