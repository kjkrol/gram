package hexes

import (
	"embed"
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/topography/relief"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// shaders is the prisms' shader.
//
//go:embed shaders/*.wgsl
var shaders embed.FS

// Sky is the sky over the prisms: its weather hazes the far off.
type Sky interface {
	Air() air.Weather
}

// prisms draws a hex board's cells (shaders/hexes.wgsl), a prism an instance.
var prisms = render.NewMeshShaderWith("hexes", render.Files(shaders, "shaders/hexes.wgsl"), []render.Uniform{
	{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1},
	{Name: "HexSize", Size: 1}, {Name: "TopPx", Size: 1}, {Name: "GridOn", Size: 1},
}).Instanced(3)

// prismVertices is how many vertices a prism takes: its top's six triangles and six faces of two.
const prismVertices = 18 + 36

// maxTop is how many pixels a side the board from above is drawn in at most, and topPx how many a
// world unit at most.
const (
	maxTop = 4096
	topPx  = 2
)

var _ render.Direct = (*Ground)(nil)

// Ground is the ground of a hex board in relief drawn on the GPU, a render.Direct at the Ground tier
// in place of the tiles: the tiles from above — the painter's, in white, without the clouds — are
// composed once (render.Still), anew when the board or the relief changes, and drawn every frame
// into an image of the world, their water glinting on; every cell stands on it as a prism to its
// top, with a face down to each lower neighbour, taking its colours from that image, lit by the
// sun, shaded by the clouds and hazed as the terrain is, writing the frame's depth.
type Ground struct {
	world  *world.Plugin
	board  *board.Plugin
	m      board.Map // the relief's map: how high the cells stand, what lies over their tiles
	relief *relief.Relief
	sky    Sky
	grid   board.Grid
	size   float32 // from a cell's centre to its corner
	still  *render.Still
	tiles  *board.Renderer // composes the tiles from above for the still
	at     [2]uint64       // the board's changes and the relief's version the still holds, 1 more
	top    *render.Image   // the still drawn, px pixels a world unit
	px     float32
	cells  []float32 // three vec4s a cell: centre and top, the six neighbours' tops
	opts   render.DrawMeshOptions
	own    map[string][]float32
	uv     map[string]any // the frame's uniforms for the still, a pixel of the image its own
}

// New is the ground of b's hex board in the relief r over w drawn on the GPU, the cells standing as
// m says, under sky.
func New(w *world.Plugin, b *board.Plugin, m board.Map, r *relief.Relief, sky Sky) *Ground {
	brd := b.Res.Logic.Board
	h := &Ground{world: w, board: b, m: m, relief: r, sky: sky, grid: brd.Grid, size: float32(brd.CellSpan()),
		own: map[string][]float32{}, uv: map[string]any{}}
	h.opts = render.DrawMeshOptions{WriteDepth: true, Vertices: prismVertices, Uniforms: map[string]any{}}
	return h
}

func (*Ground) Init(*goke.SysInit) {}

// Compose hands the frame nothing: the ground is drawn Direct.
func (*Ground) Compose(*render.Frame, camera.Camera) {}

// Tier is where the ground comes: render.Ground.
func (*Ground) Tier() render.Tier { return render.Ground }

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
	h.still.Draw(h.top, render.UniformsOf(h.uv), h.px, 0, 0, render.Light{1, 1, 1})

	u.Into(h.opts.Uniforms)
	h.set("ViewProj", tr.M[:]...)
	h.set("Eye", tr.Eye[:]...)
	h.set("Bend", tr.Bend)
	h.set("HexSize", h.size)
	h.set("TopPx", h.px)
	grid := float32(0)
	if rs := h.board.Res.Render; rs != nil && rs.ShowGridLines && h.size*cam.Zoom() >= board.MinGridCell {
		grid = 1
	}
	h.set("GridOn", grid)
	visibility := float32(0)
	if _, eyed := cam.(camera.Eyed); eyed {
		visibility = float32(air.Visibility(h.world.Scale(), h.sky.Air()))
	}
	h.set("Visibility", visibility)
	h.opts.Depth, h.opts.Images, h.opts.Instances = t.Depth, [4]*render.Image{h.top, nil, nil, nil}, h.cells
	t.Screen.DrawMesh(nil, prisms, &h.opts)
}

// refresh composes the tiles and lays out the prisms anew where the board or the relief has
// changed; false while the board has no atlas to draw from.
func (h *Ground) refresh() bool {
	atlas := h.board.Atlas()
	if atlas == nil {
		return false
	}
	brd := h.board.Res.Logic.Board
	at := [2]uint64{brd.Changes() + 1, h.relief.Version() + 1}
	if h.still != nil && at == h.at {
		return true
	}
	space := h.world.Res.Config.Space
	ww, wh := float32(space.Width), float32(space.Height)
	if h.still == nil {
		h.still = render.NewStill()
		h.tiles = board.NewRenderer(brd, atlas, fromAbove{h.m})
		h.px = min(topPx, maxTop/ww, maxTop/wh)
		h.top = render.NewImage(int(math.Ceil(float64(ww*h.px))), int(math.Ceil(float64(wh*h.px))))
	}
	h.still.Compose(ww, wh, h.tiles.Compose)
	h.cells = h.cells[:0]
	low, _ := h.relief.Extent()
	inradius := float64(h.size) * math.Sqrt(3) / 2
	h.grid.EachCell(func(c cell.ID) {
		centre := h.grid.CellCenter(c)
		top, _ := h.m.Top(c)
		h.cells = append(h.cells, float32(centre.X), float32(centre.Y), top[0], 0)
		for s := range 6 {
			a := -math.Pi/3 + float64(s)*math.Pi/3
			n, ok := h.grid.CellAt(geom.NewVec(centre.X+2*inradius*math.Cos(a), centre.Y+2*inradius*math.Sin(a)))
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

// set hands the shader the uniform name as v, kept between frames and boxed once.
func (h *Ground) set(name string, v ...float32) {
	s, ok := h.own[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		h.own[name] = s
	}
	copy(s, v)
	h.opts.Uniforms[name] = s
}

// fromAbove is the relief's map with its tiles laid flat, from above, whatever the camera: what the
// prisms compose once.
type fromAbove struct{ board.Map }

func (m fromAbove) Look() board.Look { return board.FlatLook() }
