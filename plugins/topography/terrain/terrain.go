// Package terrain draws a relief on the GPU as a mesh: every lattice corner a vertex standing at
// its height, two triangles a cell, drawn through the scene's transform (camera.SceneTransform)
// against a depth buffer, in place of the board's tiles — a render.Direct source at the Ground
// tier.
//
// The shader (shaders/, a file per thing) is built on the composer's library
// (render.NewMeshShaderWith): it reads the sun, the air and the clock the frame set, and draws the
// water with the topography's materials. The Renderer keeps the lattice in one image of quadrants
// — heights (16 bits a corner), the way the ground faces, the way to the shore — and the mesh's
// triangles, made anew as the ground or the coast changes, and reads the ground's look from
// whoever draws the board (Surface): the board painted flat, its water in layers (WaterLayers), the
// shores and the grid. What stands on the ground is tested against the depth the ground leaves;
// DrawShadows lays the shadows of what stands on it, following its rise and fall.
package terrain

import (
	"image"
	"image/color"
	"math"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"

	"embed"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// shader is the ground's: the composer's library and materials — the sun, the air, the
// topography's water — with the lattice, the light, the surface, the grid and the mesh's stages
// after them, and the uniforms they read.
var shader = render.NewMeshShaderWith("terrain",
	render.Files(shaders, "shaders/lattice.wgsl", "shaders/light.wgsl", "shaders/surface.wgsl", "shaders/grid.wgsl", "shaders/mesh.wgsl"),
	[]render.Uniform{
		{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1},
		{Name: "Cell", Size: 1}, {Name: "Corners", Size: 2}, {Name: "Low", Size: 1}, {Name: "Span", Size: 1},
		{Name: "ShoreReach", Size: 1}, {Name: "Px", Size: 1}, {Name: "AlbedoSize", Size: 2}, {Name: "WaterPx", Size: 1}, {Name: "FlowSpan", Size: 1},
		{Name: "Shadows", Size: 1}, {Name: "GridFrom", Size: 1}, {Name: "ShadePx", Size: 1}, {Name: "CloudPx", Size: 1}, {Name: "CloudFrom", Size: 1},
	})

// shadows lays the shadows of what stands on the ground (shaders/shadow.wgsl): the lattice's heights
// and the ground's transform, a patch an instance.
var shadows = render.NewMeshShaderWith("terrain shadows",
	render.Files(shaders, "shaders/lattice.wgsl", "shaders/shadow.wgsl"),
	[]render.Uniform{
		{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1}, {Name: "Look", Size: 3}, {Name: "Perspective", Size: 1},
		{Name: "Cell", Size: 1}, {Name: "Corners", Size: 2}, {Name: "Low", Size: 1}, {Name: "Span", Size: 1},
	}).Instanced(2)

// shadowPieces is how many vertices a patch's grid takes: shadow.wgsl's shadowGrid squared, six a
// piece.
const shadowPieces = 6 * 6 * 6

// bakeShade and bakeCover are the shaders baking how much of the sun reaches the ground and how
// thick the clouds stand over it into the baked image, for the ground's shader to read between
// its pixels rather than work out at every one of the screen's.
var (
	bakeShade = render.NewShaderWith("terrain shade",
		render.Files(shaders, "shaders/lattice.wgsl", "shaders/light.wgsl", "shaders/shade.wgsl"),
		[]render.Uniform{{Name: "Cell", Size: 1}, {Name: "Corners", Size: 2}, {Name: "Low", Size: 1}, {Name: "Span", Size: 1}, {Name: "Shadows", Size: 1}})
	bakeCover = render.NewShaderWith("terrain clouds", render.Files(shaders, "shaders/cover.wgsl"), nil)
)

// shadePx is how many pixels a cell the baked image holds the shade at most, the clouds at half as
// many, and maxBaked how wide it may be.
const (
	shadePx  = 8
	maxBaked = 8192
)

// shadeTurn is how far the sun turns before the shade is baked anew: its ways' dot product under
// it, a tenth of a degree.
const shadeTurn = 0.9999985

// Ground is the relief as the renderer reads it: its heights as a lattice over a square grid —
// cols by rows corners a cell apart, row by row, false off a square grid — and a count of its
// changes. topography.Relief is one.
type Ground interface {
	Lattice() (cols, rows int, cell float32, heights []float32, ok bool)
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
	Albedo, Water *render.Image
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

// Config is how the ground is drawn: whether it casts its shadows (Shadows), and the world's
// Scale, for how far one sees through the air.
type Config struct {
	Shadows bool
	Scale   world.Scale
}

// Renderer draws the ground as a mesh: a render.Direct source at the Ground tier.
type Renderer struct {
	ground  Ground
	surface Surface
	sky     Sky
	cfg     Config

	// lattice holds by lattice corner, in quadrants: heights, normals, nothing, shores
	lattice   *render.Image
	heightsAt uint64 // one more than the ground's version the heights hold; 0 none
	coastAt   uint64 // one more than the coast's version the shores hold; 0 none
	low, span float32
	buf       []byte
	mesh      *render.Indices // two triangles a cell, as the heights split them
	tris      []uint32
	plain     *render.Image // the ground's colour without an albedo: grey
	painted   Painted       // the surface as Draw last read it

	// baked holds how much of the sun reaches the ground, for the heights of shadeAt and the sun's
	// way shadeSun, and right of it how thick the clouds stand over it, baked every frame
	baked     *render.Image
	shadeAt   uint64
	shadeSun  [3]float32
	baking    render.DrawTrianglesShaderOptions
	clouding  render.DrawTrianglesShaderOptions
	bakeVerts []render.Vertex

	opts     render.DrawMeshOptions
	uniforms map[string][]float32 // the renderer's own, and boxed once for the options
	boxed    map[string]any

	casting render.DrawMeshOptions // the shadows'
	patches []float32
}

var _ render.Direct = (*Renderer)(nil)

// New is a renderer of ground looking as surface says under sky, as cfg says.
func New(ground Ground, surface Surface, sky Sky, cfg Config) *Renderer {
	r := &Renderer{ground: ground, surface: surface, sky: sky, cfg: cfg, uniforms: map[string][]float32{}, boxed: map[string]any{}}
	r.opts = render.DrawMeshOptions{WriteDepth: true, Primed: true, Uniforms: map[string]any{}}
	return r
}

// Shader is the ground's shader, for a test to compile.
func Shader() *render.Shader { return shader }

func (r *Renderer) Init(*goke.SysInit) {}

// Compose hands the frame nothing: the ground is drawn Direct.
func (r *Renderer) Compose(*render.Frame, camera.Camera) {}

// Tier is where the ground comes in the picture: render.Ground, after the sky.
func (r *Renderer) Tier() render.Tier { return render.Ground }

// Draw draws the ground into the target's screen and depth through cam — a camera with Rays —
// under the frame's uniforms u; nothing through another camera, or off a square grid.
func (r *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Screen == nil || t.Depth == nil || !r.prepare(cam, u) {
		return
	}
	albedo := r.painted.Albedo
	if albedo == nil {
		if r.plain == nil {
			r.plain = render.NewImage(1, 1)
			r.plain.Fill(color.RGBA{128, 128, 128, 255})
		}
		albedo = r.plain
	}
	r.bake(u)
	r.opts.Images = [4]*render.Image{r.lattice, albedo, r.painted.Water, r.baked}
	r.opts.Depth = t.Depth
	t.Screen.DrawMesh(r.mesh, shader, &r.opts)
}

// DrawShadows lays the shadows patches on the ground drawn last through cam, under the frame's
// uniforms u, tested against the target's depth — hidden where the ground stands before them — and
// following the ground's rise and fall; nothing off a square grid or through a camera without
// Rays.
func (r *Renderer) DrawShadows(t render.Target, cam camera.Camera, u render.Uniforms, patches []sky.Patch) {
	if t.Screen == nil || t.Depth == nil || len(patches) == 0 || !r.prepare(cam, u) {
		return
	}
	r.patches = r.patches[:0]
	for _, p := range patches {
		r.patches = append(r.patches, p.X, p.Y, p.UX, p.UY, p.Along, p.Wide, p.Fade, p.Veil)
	}
	if r.casting.Uniforms == nil {
		r.casting.Uniforms = map[string]any{}
	}
	for k, v := range r.opts.Uniforms {
		r.casting.Uniforms[k] = v
	}
	rays, _ := cam.(camera.Rays)
	f, _ := rays.Rays()
	look, persp := f.Dir, float32(0)
	if f.DDX != ([3]float32{}) || f.DDY != ([3]float32{}) {
		persp = 1
	}
	r.casting.Uniforms["Look"] = []float32{look[0], look[1], look[2]}
	r.casting.Uniforms["Perspective"] = []float32{persp}
	r.casting.Depth, r.casting.Images = t.Depth, [4]*render.Image{r.lattice, nil, nil, nil}
	r.casting.Vertices, r.casting.Instances = shadowPieces, r.patches
	t.Screen.DrawMesh(nil, shadows, &r.casting)
}

// bake brings the baked image up to date: the shade, where the ground casts its shadows, anew as
// the ground changes or the frame's sun turns by more than shadeTurn; the clouds every frame there
// are any.
func (r *Renderer) bake(u render.Uniforms) {
	cols, rows, cell, _, _ := r.ground.Lattice()
	ks, kc := bakedScale(cols, rows)
	ws, wc, h := ks*(cols-1), kc*(cols-1), ks*(rows-1)
	if r.baked == nil || r.baked.Bounds().Dx() != ws+wc || r.baked.Bounds().Dy() != h {
		r.baked = render.NewImage(ws+wc, h)
		r.shadeAt = 0
		r.bakeVerts = make([]render.Vertex, 4)
		r.baking.Uniforms, r.clouding.Uniforms = map[string]any{}, map[string]any{}
	}
	size := [2]float32{float32(cols-1) * cell, float32(rows-1) * cell}
	if cover := u.Get("Cover"); len(cover) == 1 && cover[0] > 0 {
		u.Into(r.clouding.Uniforms)
		r.bakeInto(image.Rect(ws, 0, ws+wc, kc*(rows-1)), size, bakeCover, &r.clouding)
	}
	if !r.cfg.Shadows {
		return
	}
	sun := u.Get("Sun")
	n := float32(0)
	if len(sun) == 3 {
		n = float32(math.Sqrt(float64(sun[0]*sun[0] + sun[1]*sun[1] + sun[2]*sun[2])))
	}
	if n == 0 {
		return
	}
	way := [3]float32{sun[0] / n, sun[1] / n, sun[2] / n}
	if r.shadeAt == r.heightsAt && way[0]*r.shadeSun[0]+way[1]*r.shadeSun[1]+way[2]*r.shadeSun[2] > shadeTurn {
		return
	}
	r.shadeAt, r.shadeSun = r.heightsAt, way
	u.Into(r.baking.Uniforms)
	for _, name := range []string{"Cell", "Corners", "Low", "Span"} {
		r.baking.Uniforms[name] = r.boxed[name]
	}
	r.baking.Uniforms["Shadows"] = []float32{1}
	r.baking.Images = [4]*render.Image{r.lattice, nil, nil, nil}
	r.bakeInto(image.Rect(0, 0, ws, h), size, bakeShade, &r.baking)
}

// bakeInto draws s over the part at of the baked image, its pixels spanning the lattice's cells,
// size world units, each handed its point of the world in its custom values.
func (r *Renderer) bakeInto(at image.Rectangle, size [2]float32, s *render.Shader, op *render.DrawTrianglesShaderOptions) {
	for i, c := range [4][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		r.bakeVerts[i] = render.Vertex{DstX: float32(at.Min.X) + c[0]*float32(at.Dx()), DstY: float32(at.Min.Y) + c[1]*float32(at.Dy()),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: c[0] * size[0], Custom1: c[1] * size[1]}
	}
	r.baked.SubImage(at).DrawTrianglesShader(r.bakeVerts, quad, s, op)
}

// quad is a rectangle's two triangles over its four corners, top-left, top-right, bottom-left,
// bottom-right.
var quad = []uint16{0, 1, 2, 1, 2, 3}

// bakedScale is how many pixels a cell the baked image of a lattice cols by rows corners holds the
// shade and the clouds at: shadePx and half as many, fewer where it would be wider than maxBaked.
func bakedScale(cols, rows int) (shade, clouds int) {
	shade = max(min(shadePx, 2*maxBaked/(3*max(cols-1, 1)), maxBaked/max(rows-1, 1)), 2)
	return shade, shade / 2
}

// prepare readies a draw through cam under the frame's uniforms u: the lattice, the mesh and the
// surface brought up to date, the frame's uniforms and the renderer's own over them; false where
// there is nothing to draw.
func (r *Renderer) prepare(cam camera.Camera, u render.Uniforms) bool {
	rays, ok := cam.(camera.Rays)
	if !ok {
		return false
	}
	field, ok := rays.Rays()
	if !ok || !r.refresh() {
		return false
	}
	w, h := cam.Viewport()
	t, ok := camera.SceneTransform(field, w, h)
	if !ok {
		return false
	}
	u.Into(r.opts.Uniforms)
	r.setUniforms(t, cam)
	return true
}

// refresh brings the lattice and the mesh up to date with the ground and the surface's shores,
// and reads the surface; false where there is no lattice to draw, or one too large for the image.
func (r *Renderer) refresh() bool {
	cols, rows, cell, heights, ok := r.ground.Lattice()
	if !ok || cols < 2 || rows < 2 || 2*cols > maxLattice || 2*rows > maxLattice {
		return false
	}
	if r.lattice == nil || r.lattice.Bounds().Dx() != 2*cols || r.lattice.Bounds().Dy() != 2*rows {
		r.lattice = render.NewImage(2*cols, 2*rows)
		r.heightsAt, r.coastAt = 0, 0
	}
	if v := r.ground.Version() + 1; r.heightsAt != v {
		r.heightsAt = v
		r.low, r.span, r.buf = encode(heights, r.buf[:0])
		r.write(0, 0, cols, rows)
		r.tris = triangles(cols, rows, r.buf, r.tris[:0])
		if r.mesh != nil {
			r.mesh.Release()
		}
		r.mesh = render.NewIndices(r.tris)
		r.buf = normals(cols, rows, cell, heights, r.buf[:0])
		r.write(cols, 0, cols, rows)
	}
	r.painted = r.surface.Surface()
	if v := r.painted.Coast + 1; r.coastAt != v {
		r.coastAt = v
		r.buf = shores(cols, rows, r.painted.Shores, r.painted.Reach, r.buf[:0])
		r.write(cols, rows, cols, rows)
	}
	return true
}

// triangles is the mesh of a lattice cols by rows corners, counted row by row: two triangles a
// cell, split along the diagonal whose corners stand nearer as the heights encoded in enc have
// them — as the shader's drawn splits it; appended to dst.
func triangles(cols, rows int, enc []byte, dst []uint32) []uint32 {
	at := func(i uint32) int { return int(enc[4*i])<<8 | int(enc[4*i+1]) }
	apart := func(a, b uint32) int {
		d := at(a) - at(b)
		if d < 0 {
			return -d
		}
		return d
	}
	for y := range rows - 1 {
		for x := range cols - 1 {
			i0 := uint32(y*cols + x)
			i1, i2, i3 := i0+1, i0+uint32(cols), i0+uint32(cols)+1
			if apart(i0, i3) < apart(i1, i2) {
				dst = append(dst, i0, i1, i3, i0, i3, i2)
			} else {
				dst = append(dst, i0, i1, i2, i1, i3, i2)
			}
		}
	}
	return dst
}

// maxLattice is the widest the lattice's image may be: two quadrants of corners a side.
const maxLattice = 4096

// write writes the buffer into the lattice's quadrant w by h from (x, y).
func (r *Renderer) write(x, y, w, h int) {
	r.lattice.SubImage(image.Rect(x, y, x+w, y+h)).WritePixels(r.buf)
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

// setUniforms hands the shader the transform, the lattice, the surface, the grid and how far one
// sees, over the frame's: the sun, the clouds, the clock, a world unit a screen pixel spans and
// the rest come from the frame.
func (r *Renderer) setUniforms(t camera.Transform, cam camera.Camera) {
	cols, rows, cell, _, _ := r.ground.Lattice()
	r.set("ViewProj", t.M[:]...)
	r.set("Eye", t.Eye[:]...)
	r.set("Bend", t.Bend)
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
	ks, kc := bakedScale(cols, rows)
	r.set("ShadePx", float32(ks))
	r.set("CloudPx", float32(kc))
	r.set("CloudFrom", float32(ks*(cols-1)))
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
