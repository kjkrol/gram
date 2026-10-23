// Package terrain draws a relief on the GPU as a mesh, every lattice corner a vertex at its
// height, in place of the board's tiles: a [Renderer] ([New]) is a render.Direct source in the
// opaque pass over a [Ground] (the relief), looking as its [Surface] says ([Painted]: the board
// painted flat, its water, the shores, the grid) under a [Sky], lit by the sun and the lamps the
// frame holds and shaded as the light baked it (render.GroundShade). What stands on the ground
// is tested against the depth it leaves, and the frame reads the ground's heights from it
// (render.GroundHeights).
package terrain

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/topography/internal/water"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"

	"embed"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

func init() { render.RegisterImage("GroundDetail") }

// the ground's shader for each care, mesh.wgsl's CARE
var byCare [cares]*render.Shader

func init() {
	for c := range byCare {
		byCare[c] = caring(c)
	}
}

func caring(care int) *render.Shader {
	src := render.Files(shaders, "shaders/lattice.wgsl", "shaders/light.wgsl",
		"shaders/surface.wgsl", "shaders/detail.wgsl", "shaders/sway.wgsl", "shaders/grid.wgsl",
		"shaders/mesh.wgsl")
	src = append([]byte(fmt.Sprintf("const CARE: u32 = %du;\n", care)), src...)
	return render.NewMeshShaderWith(fmt.Sprintf("terrain-care%d", care), src, []render.Uniform{
		{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1},
		{Name: "Cell", Size: 1}, {Name: "Corners", Size: 2}, {Name: "Low", Size: 1},
		{Name: "Span", Size: 1}, {Name: "ShoreReach", Size: 1}, {Name: "Px", Size: 1},
		{Name: "AlbedoSize", Size: 2}, {Name: "WaterPx", Size: 1}, {Name: "FlowSpan", Size: 1},
		{Name: "Shadows", Size: 1}, {Name: "GridFrom", Size: 1},
		{Name: "GridLine", Size: 4}, {Name: "GridWidth", Size: 1},
		{Name: "Skirt", Size: 2}, {Name: "Perspective", Size: 1},
		{Name: "HazeUp", Size: 1}, {Name: "HazeWay", Size: 3},
		{Name: "Sight", Size: 4}, {Name: "DetailPx", Size: 1},
		{Name: "Bands", Size: 1}, {Name: "ShadowTint", Size: 3}, {Name: "Warmth", Size: 1},
	})
}

// enough rings for the world's curve to bend the skirt under the horizon smoothly
const skirtRings = 12

// Ground is the relief as the renderer reads it: its heights as a lattice over a square grid,
// false off one, and a count of its changes.
type Ground interface {
	Lattice() (cols, rows int, cell float32, heights []float32, ok bool)
	Version() uint64
}

// Surface is what the ground looks like beyond its heights, brought up to date as it is asked
// for.
type Surface interface {
	Surface() Painted
}

// Painted is the ground's look: the board painted flat, its water in quadrants of water.Layers,
// the way to the shore from every lattice corner, the grid and where water may lie. A nil Albedo
// is plain grey, a nil Water none, a Grid of 0 none, a Wetness of 0 water anywhere, a nil
// Details no detail.
type Painted struct {
	Albedo, Water *render.Image
	Px, WaterPx   int // pixels a cell
	// Kinds is the detail each pixel's kind carries, Px pixels a cell as the Albedo; Details
	// the kinds' details baked (painter.DetailPx pixels a tile), nil without one; CellDetails
	// by cell the detail of its kind, or Dressed where Kinds must be read, CellDetailsAt
	// counting its changes
	Kinds, Details *render.Image
	CellDetails    []byte
	CellDetailsAt  uint64
	// Shores is the way to the shore from every corner of the lattice as the lattice holds it
	// ((way + 1) / 2 in red and green, the distance over Reach in blue), nil for open water;
	// Reach the farthest a shore is seen from
	Shores    *render.Image
	Reach     float32
	Coast     uint64     // counts the shores' changes
	Grid      float32    // screen pixels a cell spans from which the grid is drawn
	GridLine  [4]float32 // premultiplied
	GridWidth float32    // screen pixels
	Wet       []bool
	Wetness   uint64 // counts the changes of Wet
	// Takes is, by cell row by row, the shade the cell takes (render.Taking), nil for all, and
	// Sway how much what stands on it bends in the wind, nil for none; Styled counts their
	// changes
	Takes  []render.Taking
	Sway   []float32
	Styled uint64
}

// Sky is the air over the ground, for how far one sees through it.
type Sky interface {
	Air() air.Weather
}

// Config is how the ground is drawn: the world's Scale for how far one sees through the air.
type Config struct {
	Scale world.Scale
	// what a camera sees of the world's sectors, the ground drawn of those alone; nil, all of it
	Seen func(camera.Camera) *view.View
}

// Renderer draws the ground as a mesh: a render.Direct source in the opaque pass.
type Renderer struct {
	ground  Ground
	surface Surface
	sky     Sky
	cfg     Config
	hazed   bool
	shading painter.Shading

	// quadrants: heights, normals, wet cells, shores
	lattice   *render.Image
	heightsAt uint64 // one more than the ground's version the heights hold; 0 none
	coastAt   uint64
	wetAt     [3]uint64
	low, span float32
	buf       []byte
	mesh      *render.Indices
	tris      []uint32
	// the sectors the triangles are laid by, nil for none, each one's runs, the skirt's, and the
	// runs a frame draws, by care, with how many sectors each care took
	grid    view.Sectors
	spans   []sectorSpans
	skirted span
	drawn   []span
	care    int
	ranges  [cares][]render.IndexRange
	counts  [cares]int
	plain   *render.Image
	painted Painted

	opts     render.DrawMeshOptions
	uniforms map[string][]float32
	boxed    map[string]any
}

var _ render.Direct = (*Renderer)(nil)

// New is a renderer of ground looking as surface says under sky, as cfg says.
func New(ground Ground, surface Surface, sky Sky, cfg Config) *Renderer {
	r := &Renderer{ground: ground, surface: surface, sky: sky, cfg: cfg, uniforms: map[string][]float32{}, boxed: map[string]any{}}
	r.opts = render.DrawMeshOptions{WriteDepth: true, Primed: true, Uniforms: map[string]any{}}
	return r
}

// Shader is the ground's shader with full care, for a test to compile; Shaders every care's.
func Shader() *render.Shader { return byCare[FullCare] }

// Shaders are the ground's shaders from the least care to the full, for a test to compile.
func Shaders() []*render.Shader { return byCare[:] }

// SetCare has the ground within rings sectors of the one under the middle of the screen, and of
// the one under the eye, shaded with full care, and one care less every rings sectors farther
// (the cares, sectors.go), down to the least. Zero, every sector with full care.
func (r *Renderer) SetCare(rings int) { r.care = max(rings, 0) }

// Care is how many sectors out from the middle of the screen the ground is shaded with full
// care, 0 every one.
func (r *Renderer) Care() int { return r.care }

// Cared is how many sectors the last frame shaded with each care, the least first.
func (r *Renderer) Cared() [cares]int { return r.counts }

// Compose has the ground drawn Direct, in the opaque pass, and hands the frame its heights
// (render.GroundHeights).
func (r *Renderer) Compose(f *render.Frame, _ camera.Camera) {
	f.DrawOpaque()
	cols, rows, cell, _, ok := r.ground.Lattice()
	if !ok || r.lattice == nil || r.heightsAt == 0 {
		return
	}
	// the heights as the last frame drew them: they change as the relief is shaped, by hand
	f.GroundHeights(render.GroundHeights{Image: r.lattice, Corners: [2]int{cols, rows}, Cell: cell,
		Low: r.low, Span: r.span})
}

// Draw draws the ground into the target through cam under the frame's uniforms u, ring by ring
// of sectors with the care each takes. It draws nothing through a camera without Rays, or off
// a square grid.
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
	r.opts.Images = [4]*render.Image{r.lattice, albedo, r.painted.Water, r.painted.Kinds}
	r.opts.Depth = t.Depth
	r.drawn = r.runs(cam, r.drawn[:0])
	for c := range r.ranges {
		r.ranges[c] = r.ranges[c][:0]
	}
	for _, s := range r.drawn {
		r.ranges[s.care] = append(r.ranges[s.care],
			render.IndexRange{First: s.first, Count: s.count})
	}
	// each care's ground at one go, the full care first: every call primes its depth against
	// what is drawn, so the far ground the near hills hide is never shaded
	for c := cares - 1; c >= 0; c-- {
		if len(r.ranges[c]) == 0 {
			continue
		}
		r.opts.Ranges = r.ranges[c]
		t.Screen.DrawMesh(r.mesh, byCare[c], &r.opts)
	}
}

func (r *Renderer) prepare(cam camera.Camera, u render.Uniforms) bool {
	rays, ok := cam.(camera.Rays)
	if !ok {
		return false
	}
	field, ok := rays.Rays()
	if !ok || !r.refresh(cam) {
		return false
	}
	w, h := cam.Viewport()
	t, ok := camera.SceneTransform(field, w, h)
	if !ok {
		return false
	}
	u.Into(r.opts.Uniforms)
	if r.painted.Details != nil {
		r.opts.Uniforms["GroundDetail"] = r.painted.Details
	} else {
		delete(r.opts.Uniforms, "GroundDetail")
	}
	r.setUniforms(t, cam)
	r.set("Perspective", flag(perspective(field)))
	return true
}

func perspective(f camera.RayField) bool {
	return f.DDX != ([3]float32{}) || f.DDY != ([3]float32{})
}

func flag(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

func (r *Renderer) refresh(cam camera.Camera) bool {
	cols, rows, cell, heights, ok := r.ground.Lattice()
	if !ok || cols < 2 || rows < 2 || 2*cols > maxLattice || 2*rows > maxLattice {
		return false
	}
	if r.lattice == nil || r.lattice.Bounds().Dx() != 2*cols || r.lattice.Bounds().Dy() != 2*rows {
		r.lattice = render.NewImage(2*cols, 2*rows)
		r.heightsAt, r.coastAt, r.wetAt = 0, 0, [3]uint64{}
	}
	if v := r.ground.Version() + 1; r.heightsAt != v {
		r.heightsAt = v
		r.low, r.span, r.buf = encode(heights, r.buf[:0])
		r.write(0, 0, cols, rows)
		r.relay(cam, cols, rows, cell)
		r.buf = normals(cols, rows, cell, heights, r.buf[:0])
		r.write(cols, 0, cols, rows)
	} else if r.cfg.Seen != nil && r.cfg.Seen(cam).Grid() != r.grid {
		r.buf = r.buf[:0]
		_, _, r.buf = encode(heights, r.buf)
		r.relay(cam, cols, rows, cell)
	}
	r.painted = r.surface.Surface()
	if v := r.painted.Coast + 1; r.coastAt != v {
		r.coastAt = v
		if s := r.painted.Shores; s != nil {
			r.lattice.SubImage(image.Rect(cols, rows, 2*cols, 2*rows)).CopyFrom(s)
		} else {
			r.buf = openWater(cols, rows, r.buf[:0])
			r.write(cols, rows, cols, rows)
		}
	}
	v := [3]uint64{r.painted.Wetness + 1, r.painted.Styled + 1, r.painted.CellDetailsAt + 1}
	if r.wetAt != v {
		r.wetAt = v
		r.buf = flags(cols, rows, r.painted.Wet, r.painted.Wetness == 0, r.painted.Takes,
			r.painted.Sway, r.painted.CellDetails, r.buf[:0])
		r.write(0, rows, cols, rows)
	}
	return true
}

// the triangles laid anew by the sectors cam's View sees, from the heights encoded in r.buf
func (r *Renderer) relay(cam camera.Camera, cols, rows int, cell float32) {
	r.grid = nil
	if r.cfg.Seen != nil {
		r.grid = r.cfg.Seen(cam).Grid()
	}
	r.lay(cols, rows, cell, r.buf)
	if r.mesh != nil {
		r.mesh.Release()
	}
	r.mesh = render.NewIndices(r.tris)
}

// a cell's flags at its top-left corner: whether water may lie on it in red, the shade it takes
// in green, how much what stands on it sways in blue, the detail of its kind in alpha
// (painter.Dressed where its pixels differ, as they do where no kinds are given)
func flags(cols, rows int, wet []bool, all bool, takes []render.Taking, sway []float32,
	kinds []byte, dst []byte) []byte {
	for y := range rows {
		for x := range cols {
			i := y*(cols-1) + x
			inside := x < cols-1 && y < rows-1
			v := byte(0)
			if all || inside && i < len(wet) && wet[i] {
				v = 255
			}
			t := render.TakesAll
			if inside && i < len(takes) {
				t = takesOf(takes[i])
			}
			s := byte(0)
			if inside && i < len(sway) {
				s = byte(min(max(sway[i], 0), 1)*255 + 0.5)
			}
			k := byte(painter.Dressed)
			if inside && i < len(kinds) {
				k = kinds[i]
			}
			dst = append(dst, v, byte(t), s, k)
		}
	}
	return dst
}

// the shade a cell takes, every bit for a kind saying none, no bit for one that Shines
func takesOf(t render.Taking) render.Taking {
	switch {
	case t == 0:
		return render.TakesAll
	case t&render.Shines != 0:
		return 0
	}
	return t
}

// the rings' vertices are counted past the lattice's corners, ring after ring round the edge as
// rim goes (mesh.wgsl's vs_main)
func skirt(cols, rows int, dst []uint32) []uint32 {
	n := 2*(cols-1) + 2*(rows-1)
	at := func(ring, j int) uint32 {
		j %= n
		if ring == 0 {
			x, y := rim(cols, rows, j)
			return uint32(y*cols + x)
		}
		return uint32(cols*rows + (ring-1)*n + j)
	}
	for ring := range skirtRings {
		for j := range n {
			a, b, c, d := at(ring, j), at(ring, j+1), at(ring+1, j), at(ring+1, j+1)
			dst = append(dst, a, b, c, b, d, c)
		}
	}
	return dst
}

// clockwise from the top-left
func rim(cols, rows, j int) (x, y int) {
	a, b := cols-1, rows-1
	switch {
	case j < a:
		return j, 0
	case j < a+b:
		return a, j - a
	case j < 2*a+b:
		return a - (j - a - b), b
	}
	return 0, b - (j - 2*a - b)
}

const maxLattice = 4096

func (r *Renderer) write(x, y, w, h int) {
	r.lattice.SubImage(image.Rect(x, y, x+w, y+h)).WritePixels(r.buf)
}

// open water at every corner: no way to a shore, the whole reach off, as Shores holds it
func openWater(cols, rows int, dst []byte) []byte {
	for range cols * rows {
		dst = append(dst, 128, 128, 255, 255)
	}
	return dst
}

// (n+1)/2 in red, green and blue; one-sided at the edge
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

func decodeNormal(r, g, b byte) [3]float32 {
	return [3]float32{float32(r)/255*2 - 1, float32(g)/255*2 - 1, float32(b)/255*2 - 1}
}

// 16 bits a corner, red the high byte and green the low, of the way from low to the highest
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

func decode(hi, lo byte, low, span float32) float32 {
	return low + float32(uint32(hi)<<8|uint32(lo))/65535*span
}

// Haze says whether the air hazes the ground through any camera; off, it is drawn clear.
func (r *Renderer) Haze(on bool) { r.hazed = on }

// SetShading styles the light on the ground as s says.
func (r *Renderer) SetShading(s painter.Shading) { r.shading = s }

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
	r.set("FlowSpan", water.FlowSpan)
	r.set("GridFrom", r.painted.Grid)
	r.set("GridLine", r.painted.GridLine[:]...)
	r.set("GridWidth", r.painted.GridWidth)
	r.set("Skirt", skirtReach(cols, rows, cell, r.cfg.Scale), skirtRings)
	var fog [4]float32
	if r.cfg.Seen != nil {
		fog = r.cfg.Seen(cam).Fogging()
	}
	r.set("Sight", fog[:]...)
	var seeing, up float32
	var way [3]float32
	if r.sky != nil {
		seeing, up, way = r.sky.Air().Seen(cam, r.hazed)
	}
	r.set("Visibility", seeing)
	r.set("HazeUp", up)
	r.set("HazeWay", way[:]...)
	detail := float32(0)
	if r.painted.Details != nil && r.painted.Kinds != nil {
		detail = painter.DetailPx
	}
	r.set("DetailPx", detail)
	r.set("Bands", float32(r.shading.Bands))
	tint := [3]float32{1, 1, 1}
	if c := r.shading.ShadowTint; c != (color.RGBA{}) {
		tint = [3]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255}
	}
	r.set("ShadowTint", tint[:]...)
	r.set("Warmth", r.shading.Warmth)
}

// times farther from the world's middle than its edge: to the horizon seen from under the
// clouds, twenty times the world's diagonal at least, two hundred at most
func skirtReach(cols, rows int, cell float32, scale world.Scale) float32 {
	diag := math.Hypot(float64(cols-1), float64(rows-1)) * float64(cell)
	reach := 20 * diag
	if h := scale.Horizon(scale.Units(air.CloudBase)); !math.IsInf(h, 0) && h > reach {
		reach = min(h, 200*diag)
	}
	return float32(reach / (diag / 2))
}

// kept between frames and boxed once, over whatever the frame's uniforms put there
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
