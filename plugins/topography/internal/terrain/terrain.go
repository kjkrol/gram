// Package terrain draws a relief on the GPU as a mesh, every lattice corner a vertex at its
// height, in place of the board's tiles: a [Renderer] ([New]) is a render.Direct source in the
// opaque pass over a [Ground] (the relief), looking as its [Surface] says ([Painted]: the board
// painted flat, its water, the shores, the grid) under a [Sky]. What stands on the ground is
// tested against the depth it leaves; [Renderer.DrawShadows] lays the shadows of what stands on
// it. The relief's shadows and those of the world's blocks are baked as the sun moves.
package terrain

import (
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/topography/internal/water"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"

	"embed"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

var shader = render.NewMeshShaderWith("terrain",
	render.Files(shaders, "shaders/lattice.wgsl", "shaders/light.wgsl", "shaders/surface.wgsl", "shaders/grid.wgsl", "shaders/mesh.wgsl"),
	[]render.Uniform{
		{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1},
		{Name: "Cell", Size: 1}, {Name: "Corners", Size: 2}, {Name: "Low", Size: 1}, {Name: "Span", Size: 1},
		{Name: "ShoreReach", Size: 1}, {Name: "Px", Size: 1}, {Name: "AlbedoSize", Size: 2}, {Name: "WaterPx", Size: 1}, {Name: "FlowSpan", Size: 1},
		{Name: "Shadows", Size: 1}, {Name: "GridFrom", Size: 1},
		{Name: "GridLine", Size: 4}, {Name: "GridWidth", Size: 1},
		{Name: "Skirt", Size: 2}, {Name: "Perspective", Size: 1},
		{Name: "HazeUp", Size: 1}, {Name: "HazeWay", Size: 3},
	})

// enough rings for the world's curve to bend the skirt under the horizon smoothly
const skirtRings = 12

var shadows = render.NewMeshShaderWith("terrain shadows",
	render.Files(shaders, "shaders/lattice.wgsl", "shaders/shadow.wgsl"),
	[]render.Uniform{
		{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1}, {Name: "Look", Size: 3}, {Name: "Perspective", Size: 1},
		{Name: "Cell", Size: 1}, {Name: "Corners", Size: 2}, {Name: "Low", Size: 1}, {Name: "Span", Size: 1},
	}).Instanced(3)

// shadow.wgsl's shadowGrid squared, six vertices a piece
const shadowPieces = 6 * 6 * 6

var bakeShade = render.NewShaderWith("terrain shade",
	render.Files(shaders, "shaders/lattice.wgsl", "shaders/light.wgsl", "shaders/shade.wgsl"),
	[]render.Uniform{
		{Name: "Cell", Size: 1},
		{Name: "Corners", Size: 2},
		{Name: "Low", Size: 1},
		{Name: "Span", Size: 1},
		{Name: "Shadows", Size: 1},
		{Name: "ShadeSpan", Size: 1},
	})

var bakeBlocks = render.NewMeshShaderWith("terrain blocks' shade",
	render.Files(shaders, "shaders/lattice.wgsl", "shaders/light.wgsl", "shaders/blocks.wgsl"),
	[]render.Uniform{
		{Name: "Cell", Size: 1},
		{Name: "Corners", Size: 2},
		{Name: "Low", Size: 1},
		{Name: "Span", Size: 1},
		{Name: "Shadows", Size: 1},
		{Name: "ShadeSpan", Size: 1},
		{Name: "Strip", Size: 4},
	}).Instanced(2)

// pixels a cell the baked shade holds at most, and how wide the image may be
const (
	shadePx  = 8
	maxBaked = 8192
)

// dot products of the sun's ways: a tenth of a degree turned bakes the shade anew in
// shadeStrips strips, one a frame; a degree leapt bakes all of it at once
const (
	shadeTurn   = 0.9999985
	shadeStrips = 16
	shadeLeap   = 0.99985
)

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
// is plain grey, a nil Water none, a Grid of 0 none, a Wetness of 0 water anywhere.
type Painted struct {
	Albedo, Water *render.Image
	Px, WaterPx   int // pixels a cell
	Shores        []water.Shore
	Reach         float32    // the farthest a shore is seen from
	Coast         uint64     // counts the shores' changes
	Grid          float32    // screen pixels a cell spans from which the grid is drawn
	GridLine      [4]float32 // premultiplied
	GridWidth     float32    // screen pixels
	Wet           []bool
	Wetness       uint64 // counts the changes of Wet
}

// Sky is the air over the ground, for how far one sees through it, and the sun its shadows fall
// from.
type Sky interface {
	Air() air.Weather
	Sun() sky.Sun
}

// Config is how the ground is drawn: whether it casts its shadows, and the world's Scale for
// how far one sees through the air.
type Config struct {
	Shadows bool
	Scale   world.Scale
}

// Renderer draws the ground as a mesh: a render.Direct source in the opaque pass.
type Renderer struct {
	ground  Ground
	surface Surface
	sky     Sky
	cfg     Config
	hazed   bool

	// quadrants: heights, normals, wet cells, shores
	lattice   *render.Image
	heightsAt uint64 // one more than the ground's version the heights hold; 0 none
	coastAt   uint64
	wetAt     uint64
	low, span float32
	buf       []byte
	mesh      *render.Indices
	tris      []uint32
	plain     *render.Image
	painted   Painted

	// the shade baked, and as it was before each band's last bake (render.GroundShade)
	baked     *render.Image
	was       *render.Image
	fresh     bool // neither filled yet
	shown     bool // baked once since they were made
	shade     render.GroundShade
	frames    uint64                 // Compose calls
	bakedAt   [shadeStrips]uint64    // the frame each band was last baked in
	plan      struct{ from, to int } // the rows the frame's Draw bakes
	planWay   [3]float32             // the sun's way they are baked under
	shadeAt   uint64
	shadeSun  [3]float32
	strip     int
	coarse    bool
	baking    render.DrawTrianglesShaderOptions
	blocking  render.DrawMeshOptions
	bakeVerts []render.Vertex

	opts     render.DrawMeshOptions
	uniforms map[string][]float32
	boxed    map[string]any

	casting render.DrawMeshOptions

	findBlocks func(dst []block) []block // nil before Init
	blockQuery *goke.Query
	base       goke.Comp[world.Base]
	z          goke.Comp[world.Z]
	shadow     goke.Comp[world.Shadow]
	lie        goke.OptComp[world.Lie]
	found      []block
	blocks     []float32
	near       []float32
	blocksAt   uint64 // a hash of the blocks as last found; 0 none
}

// a rectangle round (x, y) running (ux, uy), along from its middle that way and wide across
type block struct{ x, y, ux, uy, along, wide, bottom, top float32 }

var _ render.Direct = (*Renderer)(nil)

// New is a renderer of ground looking as surface says under sky, as cfg says.
func New(ground Ground, surface Surface, sky Sky, cfg Config) *Renderer {
	r := &Renderer{ground: ground, surface: surface, sky: sky, cfg: cfg, uniforms: map[string][]float32{}, boxed: map[string]any{}}
	r.opts = render.DrawMeshOptions{WriteDepth: true, Primed: true, Uniforms: map[string]any{}}
	return r
}

// Shader is the ground's shader, for a test to compile.
func Shader() *render.Shader { return shader }

func (r *Renderer) Init(si *goke.SysInit) {
	r.blockQuery = si.NewQueryBuilder(&r.base, &r.z, &r.shadow).Optional(&r.lie).Build()
	r.findBlocks = r.onGround
}

// Compose has the ground drawn Direct, in the opaque pass, and hands the frame its heights
// (render.GroundHeights) and the sun on it (render.GroundShade).
func (r *Renderer) Compose(f *render.Frame, _ camera.Camera) {
	f.DrawOpaque()
	cols, rows, cell, _, ok := r.ground.Lattice()
	if !ok || r.lattice == nil || r.heightsAt == 0 {
		return
	}
	// the heights as the last frame drew them: they change as the relief is shaped, by hand
	f.GroundHeights(render.GroundHeights{Image: r.lattice, Corners: [2]int{cols, rows}, Cell: cell,
		Low: r.low, Span: r.span})
	r.planBake()
	if r.shown {
		f.GroundShade(r.shade)
	}
}

// Draw draws the ground into the target through cam under the frame's uniforms u. It draws
// nothing through a camera without Rays, or off a square grid.
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
	r.opts.Images = [4]*render.Image{r.lattice, albedo, r.painted.Water, nil}
	r.opts.Depth = t.Depth
	t.Screen.DrawMesh(r.mesh, shader, &r.opts)
}

// DrawShadows lays the shadows casts on the ground drawn last through cam, following its rise and
// fall: twelve floats a cast, a sky.Patch and the corners of the sprite on atlas whose shape it
// has, all 0 for a patch's own. It draws nothing off a square grid or through a camera without
// Rays.
func (r *Renderer) DrawShadows(
	t render.Target, cam camera.Camera, u render.Uniforms, atlas render.AtlasSource,
	casts []float32,
) {
	if t.Screen == nil || t.Depth == nil || len(casts) == 0 || !r.prepare(cam, u) {
		return
	}
	if r.casting.Uniforms == nil {
		r.casting.Uniforms = map[string]any{}
	}
	for k, v := range r.opts.Uniforms {
		r.casting.Uniforms[k] = v
	}
	rays, _ := cam.(camera.Rays)
	f, _ := rays.Rays()
	r.casting.Uniforms["Look"] = []float32{f.Dir[0], f.Dir[1], f.Dir[2]}
	r.casting.Uniforms["Perspective"] = []float32{flag(perspective(f))}
	var sheet *render.Image
	if atlas != nil {
		sheet = atlas.Atlas()
	}
	r.casting.Depth, r.casting.Images = t.Depth, [4]*render.Image{r.lattice, sheet, nil, nil}
	r.casting.Vertices, r.casting.Instances = shadowPieces, casts
	t.Screen.DrawMesh(nil, shadows, &r.casting)
}

// planBake settles what of the shade the frame's Draw bakes, and how far each band has faded
// from the shade before into its own since it was last baked.
func (r *Renderer) planBake() {
	r.frames++
	if r.plan.from < r.plan.to { // planned, never drawn: what it would have baked is missing
		r.shadeAt = 0
	}
	r.plan.from, r.plan.to = 0, 0
	cols, rows, cell, _, ok := r.ground.Lattice()
	if !r.cfg.Shadows || !ok || r.sky == nil {
		r.shown = false
		return
	}
	ks := bakedScale(cols, rows, r.coarse)
	ws, h := ks*(cols-1), ks*(rows-1)
	if r.baked == nil || r.baked.Bounds().Dx() != ws || r.baked.Bounds().Dy() != h {
		if r.baked != nil {
			r.baked.Deallocate()
			r.was.Deallocate()
		}
		r.baked, r.was = render.NewImage(ws, h), render.NewImage(ws, h)
		r.fresh, r.shown, r.shadeAt = true, false, 0
		r.bakeVerts = make([]render.Vertex, 4)
		r.baking.Uniforms = map[string]any{}
	}
	per := (h + shadeStrips - 1) / shadeStrips
	r.shade.Now, r.shade.Was = r.baked, r.was
	r.shade.Area = [4]float32{0, 0, float32(cols-1) * cell, float32(rows-1) * cell}
	r.shade.Band = float32(per) / float32(h)
	defer r.fade()
	sun := r.sky.Sun().Dir
	n := float32(math.Sqrt(float64(sun[0]*sun[0] + sun[1]*sun[1] + sun[2]*sun[2])))
	if n == 0 {
		return
	}
	way := [3]float32{sun[0] / n, sun[1] / n, sun[2] / n}
	turned := way[0]*r.shadeSun[0] + way[1]*r.shadeSun[1] + way[2]*r.shadeSun[2]
	moved := r.gather()
	switch {
	case r.shadeAt != r.heightsAt || turned < shadeLeap || moved:
		r.shadeAt, r.shadeSun, r.strip = r.heightsAt, way, 0
		r.plan.from, r.plan.to = 0, h
		for k := range r.bakedAt {
			r.bakedAt[k] = r.frames
		}
	case r.strip == 0 && turned > shadeTurn:
		return
	default:
		if r.strip == 0 {
			r.shadeSun = way
		}
		r.plan.from, r.plan.to = min(r.strip*per, h), min((r.strip+1)*per, h)
		r.bakedAt[r.strip] = r.frames
		r.strip = (r.strip + 1) % shadeStrips
	}
	if !r.shown { // the first shade baked shows at once
		for k := range r.bakedAt {
			r.bakedAt[k] = r.frames - shadeStrips
		}
	}
	r.planWay = way
	r.shade.Span = r.shadeSpan()
}

// fade has each band fade from the shade before into its own over shadeStrips frames since it was
// baked.
func (r *Renderer) fade() {
	for k, at := range r.bakedAt {
		r.shade.Mix[k] = min(float32(r.frames-at)/shadeStrips, 1)
	}
}

// bake bakes the rows the frame's plan says, under the sun r.planWay, keeping each band as it was
// before (r.was) for the frame to fade from.
func (r *Renderer) bake(u render.Uniforms) {
	if r.baked == nil {
		return
	}
	if r.fresh {
		// the sun everywhere: none of it held back, no shadow reaching up
		r.baked.Fill(color.RGBA{R: 255, B: 255, A: 255})
		r.was.Fill(color.RGBA{R: 255, B: 255, A: 255})
		r.fresh = false
	}
	y0, y1 := r.plan.from, r.plan.to
	r.plan.from, r.plan.to = 0, 0
	if y0 >= y1 {
		return
	}
	r.shown = true
	cols, rows, cell, _, _ := r.ground.Lattice()
	ws, h := r.baked.Bounds().Dx(), r.baked.Bounds().Dy()
	size := [2]float32{float32(cols-1) * cell, float32(rows-1) * cell}
	rows0 := image.Rect(0, y0, ws, y1)
	r.was.SubImage(rows0).CopyFrom(r.baked.SubImage(rows0))
	u.Into(r.baking.Uniforms)
	// the images baked into are never read while baked
	delete(r.baking.Uniforms, "GroundShade")
	delete(r.baking.Uniforms, "GroundShadeWas")
	for _, name := range []string{"Cell", "Corners", "Low", "Span"} {
		r.baking.Uniforms[name] = r.boxed[name]
	}
	r.baking.Uniforms["Shadows"] = []float32{1}
	r.baking.Uniforms["ShadeSpan"] = []float32{r.shade.Span}
	r.baking.Images = [4]*render.Image{r.lattice, nil, nil, nil}
	r.blocking.Uniforms, r.blocking.Images = r.baking.Uniforms, r.baking.Images
	r.blocking.Vertices, r.blocking.Darkest = 6, true
	per := (h + shadeStrips - 1) / shadeStrips
	for y := y0; y < y1; y += per {
		from := [2]float32{0, float32(y) / float32(h) * size[1]}
		part := [2]float32{size[0], float32(min(y+per, y1)-y) / float32(h) * size[1]}
		at := image.Rect(0, y, ws, min(y+per, y1))
		r.bakeInto(at, from, part, bakeShade, &r.baking)
		if near := r.shading(from[1], from[1]+part[1], r.planWay, cell); len(near) > 0 {
			r.baking.Uniforms["Strip"] = []float32{from[0], from[1], part[0], part[1]}
			r.blocking.Instances = near
			r.baked.SubImage(at).DrawMesh(nil, bakeBlocks, &r.blocking)
		}
	}
}

// shadeSpan is how high over the ground a shadow may reach: up the relief, or to the top of the
// highest block over its lowest ground
func (r *Renderer) shadeSpan() float32 {
	span := max(r.span, 1)
	for k := 7; k < len(r.blocks); k += 8 {
		span = max(span, r.blocks[k]-r.low)
	}
	return span
}

func (r *Renderer) shading(y0, y1 float32, sun [3]float32, cell float32) []float32 {
	r.near = r.near[:0]
	for k := 0; k+8 <= len(r.blocks); k += 8 {
		b := r.blocks[k : k+8]
		far := (b[7] - r.low) / sun[2]
		spare := far*shadowSoft + cell
		half := float32(math.Abs(float64(b[3])))*b[4] + float32(math.Abs(float64(b[2])))*b[5]
		cast := -sun[1] * far
		top, bottom := b[1]-half+min(cast, 0)-spare, b[1]+half+max(cast, 0)+spare
		if bottom >= y0 && top <= y1 {
			r.near = append(r.near, b...)
		}
	}
	return r.near
}

// light.wgsl's: how wide a shadow's penumbra is per unit towards the sun
const shadowSoft = 0.08

func (r *Renderer) gather() bool {
	if r.findBlocks == nil {
		return false
	}
	r.found = r.findBlocks(r.found[:0])
	at := hashed(r.found)
	if at == r.blocksAt {
		return false
	}
	r.blocksAt = at
	r.blocks = r.blocks[:0]
	for _, b := range joined(slices.Clone(r.found)) {
		r.blocks = append(r.blocks, b.x, b.y, b.ux, b.uy, b.along, b.wide, b.bottom, b.top)
	}
	return true
}

// a deck over its Lie from its Altitude down its Height, anything else over its box from its
// Altitude up
func (r *Renderer) onGround(dst []block) []block {
	for r.blockQuery.All(); r.blockQuery.Next(); {
		cursor := r.blockQuery.Cursor()
		bases, zs, lies := r.base.Slice(cursor), r.z.Slice(cursor), r.lie.Slice(cursor)
		for i, s := range r.shadow.Slice(cursor) {
			if s.Cast != world.Baked {
				continue
			}
			box, alt := bases[i].Pos.AABB, float32(zs[i].Altitude)
			if lies != nil && lies[i].Length > 0 {
				dst = append(dst, lying(bases[i].Pos.Center(), bases[i].Vel.Dir, lies[i],
					alt-float32(zs[i].Height), alt))
				continue
			}
			dst = append(dst, block{
				x:      float32(box.TopLeft.X + box.Size.X/2),
				y:      float32(box.TopLeft.Y + box.Size.Y/2),
				ux:     1,
				along:  float32(box.Size.X / 2),
				wide:   float32(box.Size.Y / 2),
				bottom: alt,
				top:    alt + float32(zs[i].Height),
			})
		}
	}
	return dst
}

func lying(m, dir geom.Vec, lie world.Lie, bottom, top float32) block {
	c := lie.Corners(m, dir)
	length := math.Hypot(c[1].X-c[0].X, c[1].Y-c[0].Y)
	width := math.Hypot(c[2].X-c[0].X, c[2].Y-c[0].Y)
	b := block{
		x:      float32((c[0].X + c[3].X) / 2),
		y:      float32((c[0].Y + c[3].Y) / 2),
		ux:     1,
		along:  float32(length / 2),
		wide:   float32(width / 2),
		bottom: bottom,
		top:    top,
	}
	if length > 0 {
		b.ux, b.uy = float32((c[1].X-c[0].X)/length), float32((c[1].Y-c[0].Y)/length)
	}
	return b
}

// never 0
func hashed(bs []block) uint64 {
	h := uint64(14695981039346656037)
	for _, b := range bs {
		for _, v := range [8]float32{b.x, b.y, b.ux, b.uy, b.along, b.wide, b.bottom, b.top} {
			h = (h ^ uint64(math.Float32bits(v))) * 1099511628211
		}
	}
	return max(h, 1)
}

// a bridge's deck is one block however many pieces it is laid in
func joined(bs []block) []block {
	for i := range bs {
		if bs[i].ux < 0 || bs[i].ux == 0 && bs[i].uy < 0 {
			bs[i].ux, bs[i].uy = -bs[i].ux, -bs[i].uy
		}
	}
	for again := true; again; {
		again = false
		for i := 0; i < len(bs); i++ {
			for j := i + 1; j < len(bs); j++ {
				if b, ok := join(bs[i], bs[j]); ok {
					bs[i], again = b, true
					bs = slices.Delete(bs, j, j+1)
					j--
				}
			}
		}
	}
	return bs
}

func join(a, b block) (block, bool) {
	const near = 0.01
	apart := func(x, y float32) bool { return math.Abs(float64(x-y)) > near }
	if apart(a.ux, b.ux) || apart(a.uy, b.uy) || apart(a.wide, b.wide) ||
		apart(a.bottom, b.bottom) || apart(a.top, b.top) {
		return a, false
	}
	dx, dy := b.x-a.x, b.y-a.y
	if apart(dx*a.uy-dy*a.ux, 0) {
		return a, false
	}
	s := dx*a.ux + dy*a.uy
	if s-b.along > a.along+near || s+b.along < -a.along-near {
		return a, false
	}
	lo, hi := min(-a.along, s-b.along), max(a.along, s+b.along)
	mid := (lo + hi) / 2
	a.x, a.y, a.along = a.x+a.ux*mid, a.y+a.uy*mid, (hi-lo)/2
	return a, true
}

// Coarse has the shade baked half as fine a side from the next frame on, or as fine as ever
// again.
func (r *Renderer) Coarse(on bool) { r.coarse = on }

// Coarsened reports whether the shade is baked coarse.
func (r *Renderer) Coarsened() bool { return r.coarse }

// each pixel handed its point of the world in its custom values
func (r *Renderer) bakeInto(at image.Rectangle, from, size [2]float32, s *render.Shader, op *render.DrawTrianglesShaderOptions) {
	for i, c := range [4][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		r.bakeVerts[i] = render.Vertex{DstX: float32(at.Min.X) + c[0]*float32(at.Dx()), DstY: float32(at.Min.Y) + c[1]*float32(at.Dy()),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: from[0] + c[0]*size[0], Custom1: from[1] + c[1]*size[1]}
	}
	r.baked.SubImage(at).DrawTrianglesShader(r.bakeVerts, quad, s, op)
}

var quad = []uint16{0, 1, 2, 1, 2, 3}

func bakedScale(cols, rows int, coarse bool) int {
	shade := max(min(shadePx, maxBaked/max(cols-1, 1), maxBaked/max(rows-1, 1)), 2)
	if coarse {
		shade = max(shade/2, 2)
	}
	return shade
}

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

func (r *Renderer) refresh() bool {
	cols, rows, cell, heights, ok := r.ground.Lattice()
	if !ok || cols < 2 || rows < 2 || 2*cols > maxLattice || 2*rows > maxLattice {
		return false
	}
	if r.lattice == nil || r.lattice.Bounds().Dx() != 2*cols || r.lattice.Bounds().Dy() != 2*rows {
		r.lattice = render.NewImage(2*cols, 2*rows)
		r.heightsAt, r.coastAt, r.wetAt = 0, 0, 0
	}
	if v := r.ground.Version() + 1; r.heightsAt != v {
		r.heightsAt = v
		r.low, r.span, r.buf = encode(heights, r.buf[:0])
		r.write(0, 0, cols, rows)
		r.tris = skirt(cols, rows, triangles(cols, rows, r.buf, r.tris[:0]))
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
	if v := r.painted.Wetness + 1; r.wetAt != v {
		r.wetAt = v
		r.buf = wetCells(cols, rows, r.painted.Wet, r.painted.Wetness == 0, r.buf[:0])
		r.write(0, rows, cols, rows)
	}
	return true
}

// a cell's flag at its top-left corner, in red
func wetCells(cols, rows int, wet []bool, all bool, dst []byte) []byte {
	for y := range rows {
		for x := range cols {
			v := byte(0)
			if i := y*(cols-1) + x; all || x < cols-1 && y < rows-1 && i < len(wet) && wet[i] {
				v = 255
			}
			dst = append(dst, v, 0, 0, 255)
		}
	}
	return dst
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

// split along the diagonal whose corners stand nearer, as the shader's drawn splits it
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

const maxLattice = 4096

func (r *Renderer) write(x, y, w, h int) {
	r.lattice.SubImage(image.Rect(x, y, x+w, y+h)).WritePixels(r.buf)
}

// (dir+1)/2 in red and green, the distance over reach in blue; open water where none is given
func shores(cols, rows int, from []water.Shore, reach float32, dst []byte) []byte {
	unit := func(v float32) byte { return byte(min(max(v, 0), 1)*255 + 0.5) }
	for i := range cols * rows {
		s := water.Shore{Dist: reach}
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

// Shadows says whether the ground casts shadows, from the next frame on.
func (r *Renderer) Shadows(on bool) { r.cfg.Shadows = on }

// Haze says whether the air hazes the ground through any camera; off, it is drawn clear.
func (r *Renderer) Haze(on bool) { r.hazed = on }

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
	r.set("Shadows", flag(r.cfg.Shadows))
	r.set("GridFrom", r.painted.Grid)
	r.set("GridLine", r.painted.GridLine[:]...)
	r.set("GridWidth", r.painted.GridWidth)
	r.set("Skirt", skirtReach(cols, rows, cell, r.cfg.Scale), skirtRings)
	var seeing, up float32
	var way [3]float32
	if r.sky != nil {
		seeing, up, way = r.sky.Air().Seen(cam, r.hazed)
	}
	r.set("Visibility", seeing)
	r.set("HazeUp", up)
	r.set("HazeWay", way[:]...)
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
