package hexes

import (
	"embed"
	"image/color"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
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

// Sky is the sky over the ground, whose weather hazes the far off.
type Sky interface {
	Air() air.Weather
}

var fans = render.NewMeshShaderWith("hexes", render.Files(shaders, "shaders/hexes.wgsl"),
	[]render.Uniform{
		{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1},
		{Name: "HexSize", Size: 1}, {Name: "TopPx", Size: 1}, {Name: "GridLine", Size: 4},
		{Name: "GridWidth", Size: 1},
		{Name: "HazeUp", Size: 1}, {Name: "HazeWay", Size: 3},
		{Name: "Sight", Size: 4},
		{Name: "Base", Size: 1}, {Name: "Facing", Size: 2},
		{Name: "SheetPx", Size: 1}, {Name: "Rim", Size: 4},
	}).Instanced(3)

// a sector seen as a sheet: a quad over its box at the mean height of its cells' corners,
// coloured from the board from above
var sheets = render.NewMeshShaderWith("hexes sheet", render.Files(shaders, "shaders/sheet.wgsl"),
	[]render.Uniform{
		{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1},
		{Name: "TopPx", Size: 1},
		{Name: "HazeUp", Size: 1}, {Name: "SheetPx", Size: 1}, {Name: "HazeWay", Size: 3},
		{Name: "Sight", Size: 4},
	}).Instanced(2)

// a cell's top in six triangles and a wall of two down each of its six edges; drawn at L3 and
// L4, its top alone; a sheet's two triangles
const (
	topVertices   = 18
	wholeVertices = topVertices + 36
	sheetVertices = 6
)

// the floats of a cell's instance: three vec4s
const cellFloats = 12

// pixels: a side of the board from above at most, and a world unit at most
const (
	maxTop = 4096
	topPx  = 2
)

// texels a side of the image of the ground's facing at most
const maxFacing = 4096

var _ render.Direct = (*Ground)(nil)

// Ground is the ground of a hex board in relief drawn on the GPU, a render.Direct in the opaque
// pass in place of the tiles: every cell six triangles from its middle to its corners, walls
// down from the board's edge, coloured from the tiles and the ways drawn from above.
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
	cols    int
	rows    int
	size    float32 // from a cell's centre to its corner
	tiles   *look.Renderer
	at      [2]uint64
	top     *render.Image
	stale   bool    // the tops drawn before the board or the relief last changed
	px      float32 // pixels a world unit
	// three vec4s a cell: centre, level and the shade it takes; the corners' heights, the edges
	// on the board's edge and its ordinal
	cells []float32
	base  float32 // where the walls down the board's edge end
	// the way the ground faces at every corner, a texel each in the relief's order, and the
	// slopes summed round them as it is made: x, y and how many
	facing *render.Image
	slopes []float64
	pix    []byte
	// the sectors the cells are laid by, nil for none, each one's run of them and the mean height
	// of its cells' corners; the sheets a frame draws and how many sectors it drew at each level
	sectors view.Sectors
	runs    []run
	heights []float32
	sheet   []float32
	counts  [lod.Levels]int
	opts    render.DrawMeshOptions
	own     map[string]any
	uv      map[string]any
	pixel   any // the top's Pixel, a []float32 of one
}

// New is the ground of b's hex board over w, as the frame drawn holds it: the cells cells on the
// relief ground gives, their tops drawn in m's look, hazed and gridded as drawn says, under sky,
// the ways across their tops drawn by p.
func New(w *world.Plugin, b *board.Plugin, cells look.Board, m look.Map,
	ground func() *relief.Relief, drawn func() (hazed bool, grid look.RenderState), sky Sky,
	p *painter.Painter) *Ground {
	sq, _ := cells.Shape()
	h := &Ground{world: w, board: b, cellsOf: cells, m: m, ground: ground, drawn: drawn, sky: sky,
		painter: p, grid: cells, cols: int(sq.Cols), rows: int(sq.Rows),
		size: b.Read(0).Board().CellSpan(), own: map[string]any{}, uv: map[string]any{},
		pixel: []float32{0}}
	h.opts = render.DrawMeshOptions{
		WriteDepth: true,
		Vertices:   wholeVertices,
		Uniforms:   map[string]any{},
	}
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
	hazed, _ := h.drawn()
	seeing, up, way := h.sky.Air().Seen(cam, hazed)
	h.set("Visibility", seeing)
	h.set("HazeUp", up)
	h.set("HazeWay", way[:]...)
	seen := h.world.Read(0).ViewFor(cam)
	fog := seen.Fogging()
	h.set("Sight", fog[:]...)
	h.set("Base", h.base)
	h.set("Facing", float32(h.cols), float32(h.facing.Bounds().Dx()))
	h.setRim()
	h.opts.Depth, h.opts.Images = t.Depth, [4]*render.Image{h.top, h.facing, nil, nil}
	albedo, _, albedoL5, _, perUnit := h.world.Read(0).Sheet()
	h.each(seen, func(cells []float32, vertices int, level lod.Level) {
		// the grid at Standard and finer alone, whose cells are drawn whole
		grid := width
		if vertices != wholeVertices {
			grid = 0
		}
		h.set("GridWidth", grid)
		// the sheet of what stands laid over the tops at L4
		img, px := albedo, perUnit
		if level != lod.Strategic {
			img, px = nil, 0
		}
		h.laySheet(img, px)
		h.opts.Instances, h.opts.Vertices = cells, vertices
		t.Screen.DrawMesh(nil, fans, &h.opts)
	})
	if len(h.sheet) > 0 {
		h.drawSheets(t, albedoL5, perUnit/2)
	}
	delete(h.opts.Uniforms, world.SheetImage)
}

// drawSheets draws the sheets each laid, the albedo of the sheet of what stands at L5 laid over
// them from img, perUnit texels a world unit
func (h *Ground) drawSheets(t render.Target, img *render.Image, perUnit float32) {
	h.laySheet(img, perUnit)
	h.opts.Instances, h.opts.Vertices = h.sheet, sheetVertices
	t.Screen.DrawMesh(nil, sheets, &h.opts)
}

// laySheet has what is drawn next lay the albedo of the sheet of what stands from img, perUnit
// texels a world unit; nothing for none
func (h *Ground) laySheet(img *render.Image, perUnit float32) {
	if img == nil || perUnit <= 0 {
		h.set("SheetPx", 0)
		delete(h.opts.Uniforms, world.SheetImage)
		return
	}
	h.set("SheetPx", perUnit)
	h.opts.Uniforms[world.SheetImage] = img
}

// Levels is how many sectors the last frame drew at each level.
func (h *Ground) Levels() [lod.Levels]int { return h.counts }

// a sector's cells among the instances, from first, n of them
type run struct{ first, n int }

// calls draw with the cells seen at a level, each sector's whole cells at Standard or finer or
// their tops at L3 and L4, sectors following one another drawn alike at one go, the close levels
// as Standard, and lays the sheets of those at L5; every cell whole in a world of no sectors
func (h *Ground) each(seen *view.View, draw func(cells []float32, vertices int, level lod.Level)) {
	h.counts, h.sheet = [lod.Levels]int{}, h.sheet[:0]
	if h.sectors == nil {
		draw(h.cells, wholeVertices, lod.Standard)
		return
	}
	var at run
	vertices, drawn := 0, lod.Standard
	flush := func() {
		if at.n > 0 {
			draw(h.cells[cellFloats*at.first:cellFloats*(at.first+at.n)], vertices, drawn)
		}
	}
	seen.Sectors(func(i int, box geom.AABB, level lod.Level) {
		r := h.runs[i]
		if r.n == 0 {
			return
		}
		h.counts[level]++
		if level >= lod.L5 {
			h.sheet = append(h.sheet, box.TopLeft.X, box.TopLeft.Y, box.BottomRight.X,
				box.BottomRight.Y, h.heights[i], 0, 0, 0)
			return
		}
		v, as := wholeVertices, max(level, lod.Standard)
		if level > lod.Standard {
			v = topVertices
		}
		if v == vertices && as == drawn && at.first+at.n == r.first {
			at.n += r.n
			return
		}
		flush()
		at, vertices, drawn = r, v, as
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

// false while the board has no atlas to draw from, or has more corners than the GPU's images
// hold the facing of
func (h *Ground) refresh() bool {
	atlas := h.board.Read(0).Atlas()
	if atlas == nil || relief.HexCount(h.cols, h.rows) > maxFacing*maxFacing {
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
	if at[1] != h.at[1] {
		h.face()
	}
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

// the rectangle the board keeps to, which cuts the cells of its hem
func (h *Ground) setRim() {
	rim := relief.HexRim(h.cols, h.rows, h.size)
	h.set("Rim", rim.TopLeft.X, rim.TopLeft.Y, rim.BottomRight.X, rim.BottomRight.Y)
}

// an instance: where it is drawn from, the sector it is drawn with and its floats
type laid struct {
	at     geom.Vec
	floats [cellFloats]float32
}

// the cells' instances and those of the hem, the parts of the cells beside the board that fill
// its rectangle, sector after sector where the world has them
func (h *Ground) lay() {
	h.cells = h.cells[:0]
	ground := h.ground()
	low, _ := ground.Extent()
	h.base = min(low, 0)
	rim := relief.HexRim(h.cols, h.rows, h.size)
	eps := 1e-3 * h.size
	// a cell in a column of a row, on the board or beside it, as the rectangle cuts it
	cut := func(col, row int, corners [6]float32, level float32, takes render.Taking) (laid, bool) {
		q := col - row>>1
		mid := geom.NewVec(h.size*(f32.Sqrt(3)*float32(q)+f32.Sqrt(3)/2*float32(row))+h.size,
			h.size*1.5*float32(row)+h.size)
		var at [6]geom.Vec
		var sum geom.Vec
		for k, w := range ways {
			p := geom.NewVec(mid.X+h.size*float32(w[0]), mid.Y+h.size*float32(w[1]))
			at[k] = geom.NewVec(min(max(p.X, rim.TopLeft.X), rim.BottomRight.X),
				min(max(p.Y, rim.TopLeft.Y), rim.BottomRight.Y))
			sum = geom.NewVec(sum.X+at[k].X/6, sum.Y+at[k].Y/6)
		}
		// an edge on the rectangle's own is the board's edge: a wall down it
		rimmed, area := 0, float32(0)
		for k := range at {
			a, b := at[k], at[(k+1)%6]
			area += a.X*b.Y - b.X*a.Y
			if f32.Abs(a.X-b.X)+f32.Abs(a.Y-b.Y) < eps {
				continue
			}
			for _, side := range [4][3]float32{
				{a.X, b.X, rim.TopLeft.X},
				{a.X, b.X, rim.BottomRight.X},
				{a.Y, b.Y, rim.TopLeft.Y},
				{a.Y, b.Y, rim.BottomRight.Y},
			} {
				if f32.Abs(side[0]-side[2]) < eps && f32.Abs(side[1]-side[2]) < eps {
					rimmed |= 1 << k
				}
			}
		}
		if f32.Abs(area) < eps*h.size {
			return laid{}, false
		}
		c := geom.NewVec(min(max(mid.X, rim.TopLeft.X), rim.BottomRight.X),
			min(max(mid.Y, rim.TopLeft.Y), rim.BottomRight.Y))
		l := laid{at: sum}
		l.floats = [cellFloats]float32{c.X, c.Y, level, float32(takes),
			corners[0], corners[1], corners[2], corners[3], corners[4], corners[5],
			float32(rimmed), float32((row+1)*(h.cols+2) + col + 1)}
		return l, true
	}
	var all []laid
	h.grid.EachCell(func(c cell.ID) {
		col, row, _ := h.grid.Coords(c)
		corners, level, _ := ground.Hex(c)
		if l, ok := cut(int(col), int(row), corners, level, takesOf(h.cellsOf.Kind(c).Takes)); ok {
			all = append(all, l)
		}
	})
	for row := -1; row <= h.rows; row++ {
		for col := -1; col <= h.cols; col++ {
			if row >= 0 && row < h.rows && col >= 0 && col < h.cols {
				continue
			}
			corners, level, ok := ground.Hem(col, row)
			if !ok {
				continue
			}
			// it takes the light as the board's cell beside it does
			beside := h.grid.At(uint32(min(max(col, 0), h.cols-1)), uint32(min(max(row, 0), h.rows-1)))
			if l, ok := cut(col, row, corners, level, takesOf(h.cellsOf.Kind(beside).Takes)); ok {
				all = append(all, l)
			}
		}
	}
	if h.sectors == nil {
		for _, l := range all {
			h.cells = append(h.cells, l.floats[:]...)
		}
		return
	}
	by := make([][]int, h.sectors.Len())
	for k, l := range all {
		i := h.sectors.Of(l.at.X, l.at.Y)
		by[i] = append(by[i], k)
	}
	h.runs = append(h.runs[:0], make([]run, len(by))...)
	h.heights = append(h.heights[:0], make([]float32, len(by))...)
	for i, ks := range by {
		h.runs[i] = run{len(h.cells) / cellFloats, len(ks)}
		for _, k := range ks {
			h.cells = append(h.cells, all[k].floats[:]...)
			// a cell's level is the mean of its six corners
			h.heights[i] += all[k].floats[2] / float32(len(ks))
		}
	}
}

// the ways from a cell's middle to its corners, clockwise from the top
var ways = [6][2]float64{
	{0, -1},
	{math.Sqrt(3) / 2, -0.5},
	{math.Sqrt(3) / 2, 0.5},
	{0, 1},
	{-math.Sqrt(3) / 2, 0.5},
	{-math.Sqrt(3) / 2, -0.5},
}

// face has facing hold the way the ground faces at every corner, (n+1)/2 in red, green and blue
// as the terrain holds its corners': the mean of the slopes of the triangles round it, so the
// light runs on smoothly from triangle to triangle and from cell to cell
func (h *Ground) face() {
	ground := h.ground()
	n := relief.HexCount(h.cols, h.rows)
	h.slopes = slices.Grow(h.slopes[:0], 3*n)[:3*n]
	clear(h.slopes)
	s := float64(h.size)
	cross := s * s * math.Sqrt(3) / 2 // of a triangle's sides from the middle, 60° apart
	h.grid.EachCell(func(c cell.ID) {
		q, r, _ := h.grid.Coords(c)
		corners, level, _ := ground.Hex(c)
		at := relief.HexCorners(h.cols, int(q), int(r))
		for k := range 6 {
			// the triangle's slope g rises a over the side s·u, b over s·v
			a, b := float64(corners[k]-level), float64(corners[(k+1)%6]-level)
			u, v := ways[k], ways[(k+1)%6]
			gx, gy := s*(a*v[1]-b*u[1])/cross, s*(b*u[0]-a*v[0])/cross
			for _, i := range [2]int{at[k], at[(k+1)%6]} {
				h.slopes[3*i] += gx
				h.slopes[3*i+1] += gy
				h.slopes[3*i+2]++
			}
		}
	})
	wide := min(n, maxFacing)
	rows := (n + wide - 1) / wide
	h.pix = h.pix[:0]
	for i := range wide * rows {
		nx, ny, nz := 0.0, 0.0, 1.0
		if i < n && h.slopes[3*i+2] > 0 {
			k := h.slopes[3*i+2]
			nx, ny = -h.slopes[3*i]/k, -h.slopes[3*i+1]/k
		}
		l := math.Sqrt(nx*nx + ny*ny + nz*nz)
		h.pix = append(h.pix, unit(nx/l), unit(ny/l), unit(nz/l), 255)
	}
	if h.facing == nil || h.facing.Bounds().Dx() != wide || h.facing.Bounds().Dy() != rows {
		h.facing = render.NewImage(wide, rows)
	}
	h.facing.WritePixels(h.pix)
}

// a unit's component as a byte, (v+1)/2
func unit(v float64) byte { return byte((v+1)/2*255 + 0.5) }

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
