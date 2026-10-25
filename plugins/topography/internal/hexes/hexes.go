package hexes

import (
	"embed"
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/topography/internal/painter"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// Sky is the sky over the prisms, whose weather hazes the far off.
type Sky interface {
	Air() air.Weather
}

var prisms = render.NewMeshShaderWith("hexes", render.Files(shaders, "shaders/hexes.wgsl"), []render.Uniform{
	{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1},
	{Name: "HexSize", Size: 1}, {Name: "TopPx", Size: 1}, {Name: "GridLine", Size: 4},
	{Name: "GridWidth", Size: 1},
	{Name: "HazeUp", Size: 1}, {Name: "HazeWay", Size: 3},
	{Name: "Sight", Size: 4},
}).Instanced(3)

// a prism's top in six triangles and six faces of two; drawn coarsely, its top alone
const (
	topVertices   = 18
	prismVertices = topVertices + 36
)

// the floats of a cell's instance: three vec4s
const cellFloats = 12

// pixels: a side of the board from above at most, and a world unit at most
const (
	maxTop = 4096
	topPx  = 2
)

var _ render.Direct = (*Ground)(nil)

// Ground is the ground of a hex board in relief drawn on the GPU, a render.Direct in the opaque
// pass in place of the tiles: every cell a prism to its top, coloured from the tiles and the ways
// drawn from above.
type Ground struct {
	world   *world.Plugin
	board   *board.Plugin
	cellsOf look.Board // the board's cells as the frame drawn holds them
	m       look.Map
	ground  func() *relief.Relief
	drawn   func() (hazed bool, grid look.RenderState)
	sky     Sky
	painter *painter.Painter
	grid    grid.Grid
	size    float32 // from a cell's centre to its corner
	tiles   *look.Renderer
	at      [2]uint64
	top     *render.Image
	stale   bool      // the tops drawn before the board or the relief last changed
	px      float32   // pixels a world unit
	cells   []float32 // three vec4s a cell: centre and top, the six neighbours' tops
	// the sectors the cells are laid by, nil for none, and each one's run of them
	sectors view.Sectors
	runs    []run
	opts    render.DrawMeshOptions
	own     map[string]any
	uv      map[string]any
	pixel   any // the top's Pixel, a []float32 of one
}

// New is the ground of b's hex board over w, as the frame drawn holds it: the cells cells, standing
// as m says on the relief ground gives, hazed and gridded as drawn says, under sky, the ways
// across their tops drawn by p.
func New(w *world.Plugin, b *board.Plugin, cells look.Board, m look.Map,
	ground func() *relief.Relief, drawn func() (hazed bool, grid look.RenderState), sky Sky,
	p *painter.Painter) *Ground {
	h := &Ground{world: w, board: b, cellsOf: cells, m: m, ground: ground, drawn: drawn, sky: sky,
		painter: p, grid: cells, size: b.Read(0).Board().CellSpan(),
		own: map[string]any{}, uv: map[string]any{}, pixel: []float32{0}}
	h.opts = render.DrawMeshOptions{WriteDepth: true, Vertices: prismVertices, Uniforms: map[string]any{}}
	return h
}

// Compose has the ground drawn Direct, in the opaque pass.
func (*Ground) Compose(f *render.Frame, _ camera.Camera) { f.DrawOpaque() }

// Draw draws the ground into the target through cam, under the frame's uniforms u.
func (h *Ground) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Screen == nil || t.Depth == nil {
		return
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	field, ok := rays.Rays()
	if !ok {
		return
	}
	w, hgt := cam.Viewport()
	tr, ok := camera.SceneTransform(field, w, hgt)
	if !ok || !h.refresh() {
		return
	}
	if g := h.world.Read(0).ViewFor(cam).Grid(); g != h.sectors {
		h.sectors = g
		h.lay()
	}
	u.Into(h.uv)
	h.pixel.([]float32)[0] = 1 / h.px
	h.uv["Pixel"] = h.pixel
	if h.stale || h.tiles.Animated() || h.painter.WaysMoving() {
		h.drawTop(render.UniformsOf(h.uv))
	}

	u.Into(h.opts.Uniforms)
	h.set("ViewProj", tr.M[:]...)
	h.set("Eye", tr.Eye[:]...)
	h.set("Bend", tr.Bend)
	h.set("HexSize", h.size)
	h.set("TopPx", h.px)
	line, width := h.gridOf(cam)
	h.set("GridLine", line[:]...)
	h.set("GridWidth", width)
	hazed, _ := h.drawn()
	seeing, up, way := h.sky.Air().Seen(cam, hazed)
	h.set("Visibility", seeing)
	h.set("HazeUp", up)
	h.set("HazeWay", way[:]...)
	seen := h.world.Read(0).ViewFor(cam)
	fog := seen.Fogging()
	h.set("Sight", fog[:]...)
	h.opts.Depth, h.opts.Images = t.Depth, [4]*render.Image{h.top, nil, nil, nil}
	h.each(seen, func(cells []float32, vertices int) {
		h.opts.Instances, h.opts.Vertices = cells, vertices
		t.Screen.DrawMesh(nil, prisms, &h.opts)
	})
}

// a sector's cells among the instances, from first, n of them
type run struct{ first, n int }

// calls draw with the cells seen, each sector's whole prisms or, coarsely, their tops, sectors
// following one another at one level at one go; every cell whole in a world of no sectors
func (h *Ground) each(seen *view.View, draw func(cells []float32, vertices int)) {
	if h.sectors == nil {
		draw(h.cells, prismVertices)
		return
	}
	var at run
	vertices := 0
	flush := func() {
		if at.n > 0 {
			draw(h.cells[cellFloats*at.first:cellFloats*(at.first+at.n)], vertices)
		}
	}
	seen.Sectors(func(i int, _ geom.AABB, level view.Level) {
		r := h.runs[i]
		if r.n == 0 {
			return
		}
		v := prismVertices
		if level != view.Exact {
			v = topVertices
		}
		if v == vertices && at.first+at.n == r.first {
			at.n += r.n
			return
		}
		flush()
		at, vertices = r, v
	})
	flush()
}

// Miniature is the board's tops as last drawn from above, px pixels a world unit; nil before the
// board's atlas is closed.
func (h *Ground) Miniature() (*render.Image, float32) {
	if !h.refresh() {
		return nil, 0
	}
	return h.top, h.px
}

// false while the board has no atlas to draw from
func (h *Ground) refresh() bool {
	atlas := h.board.Read(0).Atlas()
	if atlas == nil {
		return false
	}
	at := [2]uint64{h.cellsOf.Changes() + 1, h.ground().Version() + 1}
	if h.tiles != nil && at == h.at {
		return true
	}
	if h.tiles == nil {
		space := h.world.Res.Config.Space
		ww, wh := float32(space.Width), float32(space.Height)
		h.tiles = look.NewRenderer(h.cellsOf, atlas, h.m, space)
		h.px = min(topPx, maxTop/ww, maxTop/wh)
		h.top = render.NewImage(int(f32.Ceil(ww*h.px)), int(f32.Ceil(wh*h.px)))
	}
	h.lay()
	h.at, h.stale = at, true
	return true
}

// drawTop draws the tops from above, px pixels a world unit, under the uniforms u: the tiles,
// then the ways across them
func (h *Ground) drawTop(u render.Uniforms) {
	h.painter.Above()
	h.top.Fill(color.Transparent)
	h.tiles.Whole(h.top, h.px, render.Light{1, 1, 1}, u)
	h.painter.DrawWays(h.top, h.board.Read(0).Atlas(), h.px, u)
	h.stale = false
}

// the cells' instances, sector after sector where the world has them
func (h *Ground) lay() {
	h.cells = h.cells[:0]
	low, _ := h.ground().Extent()
	across := h.edges()
	add := func(c cell.ID) {
		centre := h.grid.CellCenter(c)
		top, _ := h.m.Top(c)
		h.cells = append(h.cells, centre.X, centre.Y, top[0],
			float32(takesOf(h.cellsOf.Kind(c).Takes)))
		for _, i := range across {
			n, ok := h.grid.Toward(c, i)
			lowest := min(low, 0)
			if ok {
				nt, _ := h.m.Top(n)
				lowest = nt[0]
			}
			h.cells = append(h.cells, lowest)
		}
		h.cells = append(h.cells, 0, 0)
	}
	if h.sectors == nil {
		h.grid.EachCell(add)
		return
	}
	by := make([][]cell.ID, h.sectors.Len())
	h.grid.EachCell(func(c cell.ID) {
		centre := h.grid.CellCenter(c)
		i := h.sectors.Of(centre.X, centre.Y)
		by[i] = append(by[i], c)
	})
	h.runs = append(h.runs[:0], make([]run, len(by))...)
	for i, cs := range by {
		h.runs[i] = run{len(h.cells) / cellFloats, len(cs)}
		for _, c := range cs {
			add(c)
		}
	}
}

// edge k joins corners k and k+1, as the shader reads the neighbours
func (h *Ground) edges() [6]int {
	var out [6]int
	c := h.grid.At(0, 0)
	for i := range h.grid.Directions() {
		a, b := grid.Rim(h.grid, c, i)
		if b == (a+1)%6 {
			out[a] = i
		} else {
			out[b] = i
		}
	}
	return out
}

// gridOf is the board's grid's line through cam, premultiplied, and its width; 0 wide where the
// grid is hidden or its cells too small to show it
func (h *Ground) gridOf(cam camera.Camera) (line [4]float32, width float32) {
	_, drawnGrid := h.drawn()
	line, width, shown := drawnGrid.Grid()
	if !shown || h.size*cam.Zoom() < look.MinGridCell {
		return line, 0
	}
	return line, width
}

func (h *Ground) set(name string, v ...float32) {
	u, ok := h.own[name]
	if s, _ := u.([]float32); !ok || len(s) != len(v) {
		u = make([]float32, len(v))
		h.own[name] = u
	}
	copy(u.([]float32), v)
	h.opts.Uniforms[name] = u
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
