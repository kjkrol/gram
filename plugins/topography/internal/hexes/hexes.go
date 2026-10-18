package hexes

import (
	"embed"
	"image/color"
	"math"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/world"
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
}).Instanced(3)

// a prism's top in six triangles and six faces of two
const prismVertices = 18 + 36

// pixels: a side of the board from above at most, and a world unit at most
const (
	maxTop = 4096
	topPx  = 2
)

var _ render.Direct = (*Ground)(nil)

// Ground is the ground of a hex board in relief drawn on the GPU, a render.Direct in the opaque
// pass in place of the tiles: every cell a prism to its top, coloured from the tiles composed
// once from above.
type Ground struct {
	world  *world.Plugin
	board  *board.Plugin
	m      board.Map
	relief *relief.Relief
	sky    Sky
	grid   grid.Grid
	hazed  bool
	size   float32 // from a cell's centre to its corner
	still  *render.Still
	tiles  *look.Renderer
	at     [2]uint64
	top    *render.Image
	px     float32   // pixels a world unit
	cells  []float32 // three vec4s a cell: centre and top, the six neighbours' tops
	opts   render.DrawMeshOptions
	own    map[string][]float32
	uv     map[string]any
}

// New is the ground of b's hex board in the relief r over w, the cells standing as m says,
// under sky.
func New(w *world.Plugin, b *board.Plugin, m board.Map, r *relief.Relief, sky Sky) *Ground {
	brd := b.Board()
	h := &Ground{world: w, board: b, m: m, relief: r, sky: sky, grid: brd.Grid, size: float32(brd.CellSpan()),
		own: map[string][]float32{}, uv: map[string]any{}}
	h.opts = render.DrawMeshOptions{WriteDepth: true, Vertices: prismVertices, Uniforms: map[string]any{}}
	return h
}

func (*Ground) Init(*goke.SysInit) {}

// Compose has the ground drawn Direct, in the opaque pass.
func (*Ground) Compose(f *render.Frame, _ camera.Camera) { f.DrawOpaque() }

// Haze says whether the air hazes the prisms through any camera; off, they are drawn clear.
func (h *Ground) Haze(on bool) { h.hazed = on }

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
	u.Into(h.uv)
	h.uv["Pixel"] = []float32{1 / h.px}
	h.top.Fill(color.Transparent)
	top := render.Target{Screen: h.top}
	h.still.Draw(top, nil, render.UniformsOf(h.uv), h.px, 0, 0, render.Light{1, 1, 1})

	u.Into(h.opts.Uniforms)
	h.set("ViewProj", tr.M[:]...)
	h.set("Eye", tr.Eye[:]...)
	h.set("Bend", tr.Bend)
	h.set("HexSize", h.size)
	h.set("TopPx", h.px)
	line, width := h.gridOf(cam)
	h.set("GridLine", line[:]...)
	h.set("GridWidth", width)
	seeing, up, way := h.sky.Air().Seen(cam, h.hazed)
	h.set("Visibility", seeing)
	h.set("HazeUp", up)
	h.set("HazeWay", way[:]...)
	h.opts.Depth, h.opts.Images, h.opts.Instances = t.Depth, [4]*render.Image{h.top, nil, nil, nil}, h.cells
	t.Screen.DrawMesh(nil, prisms, &h.opts)
}

// false while the board has no atlas to draw from
func (h *Ground) refresh() bool {
	atlas := h.board.Atlas()
	if atlas == nil {
		return false
	}
	brd := h.board.Board()
	at := [2]uint64{brd.Changes() + 1, h.relief.Version() + 1}
	if h.still != nil && at == h.at {
		return true
	}
	space := h.world.Res.Config.Space
	ww, wh := float32(space.Width), float32(space.Height)
	if h.still == nil {
		h.still = render.NewStill()
		h.tiles = look.NewRenderer(brd, atlas, fromAbove{h.m}, world.SpaceCfg{})
		h.px = min(topPx, maxTop/ww, maxTop/wh)
		h.top = render.NewImage(int(math.Ceil(float64(ww*h.px))), int(math.Ceil(float64(wh*h.px))))
	}
	h.still.Compose(ww, wh, h.tiles.Compose)
	h.cells = h.cells[:0]
	low, _ := h.relief.Extent()
	across := h.edges()
	h.grid.EachCell(func(c cell.ID) {
		centre := h.grid.CellCenter(c)
		top, _ := h.m.Top(c)
		h.cells = append(h.cells, float32(centre.X), float32(centre.Y), top[0], 0)
		for _, i := range across {
			n, ok := h.grid.Toward(c, i)
			lowest := float32(min(low, 0))
			if ok {
				nt, _ := h.m.Top(n)
				lowest = nt[0]
			}
			h.cells = append(h.cells, lowest)
		}
		h.cells = append(h.cells, 0, 0)
	})
	h.at = at
	return true
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
	line, width, shown := h.board.Res.Render.Grid()
	if !shown || h.size*cam.Zoom() < look.MinGridCell {
		return line, 0
	}
	return line, width
}

func (h *Ground) set(name string, v ...float32) {
	s, ok := h.own[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		h.own[name] = s
	}
	copy(s, v)
	h.opts.Uniforms[name] = s
}

type fromAbove struct{ board.Map }

func (m fromAbove) Look() look.Look { return look.FlatLook() }
