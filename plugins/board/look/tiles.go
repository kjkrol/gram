package look

import (
	"image"
	"math"

	"github.com/kjkrol/gram/camera"
	_ "github.com/kjkrol/gram/plugins/atmosphere/air" // the Wind the shaders lean the tiles in
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// the passes laying the tiles: pixel by pixel the plain cells, those something sways on or beside
// or a material lies over and every hex (the full pass) and the cells drawn by their material;
// the covers and the ways an instance a cell
var (
	plainShader    = screenShader("board tiles, plain", "shaders/tiles_plain.wgsl")
	fullShader     = screenShader("board tiles", "shaders/tiles.wgsl")
	materialShader = screenShader("board tiles' materials", "shaders/tiles_material.wgsl")
	coversShader   = render.NewMeshShaderWith("board covers",
		render.Files(shaders, "shaders/tiles_common.wgsl", "shaders/tiles_covers.wgsl"),
		tilesUniforms).Instanced(1)
	waysShader = render.NewMeshShaderWith("board ways",
		render.Files(shaders, "shaders/tiles_common.wgsl", "shaders/tiles_ways.wgsl"),
		tilesUniforms).Instanced(1)
)

func screenShader(label, file string) *render.Shader {
	return render.NewMeshShaderWith(label, render.Files(shaders, "shaders/tiles_common.wgsl",
		"shaders/tiles_screen.wgsl", file), tilesUniforms)
}

var tilesUniforms = []render.Uniform{
	{Name: "ViewAt", Size: 2},
	{Name: "RayAt", Size: 3},
	{Name: "RayDX", Size: 3},
	{Name: "RayDY", Size: 3},
	{Name: "RayDir", Size: 3},
	{Name: "RayDDX", Size: 3},
	{Name: "RayDDY", Size: 3},
	{Name: "TileViewProj", Size: 16},
	{Name: "TileDeep", Size: 1},
	{Name: "TileShift", Size: 2},
	{Name: "TileHex", Size: 1},
	{Name: "TileBox", Size: 2},
	{Name: "TileSide", Size: 1},
	{Name: "TileCells", Size: 2},
	{Name: "TileWrap", Size: 2},
	{Name: "TileLit", Size: 1},
	{Name: "TileTint", Size: 3},
	{Name: "TileSways", Size: 1},
	{Name: "TileFull", Size: 1},
	{Name: "TileCovers", Size: 4},
	{Name: "TileCoversOn", Size: 4},
	{Name: "TileSeen", Size: 1},
	{Name: "TileSectors", Size: 4},
	{Name: "TileFogging", Size: 4},
}

// the bytes of a cell's block of 2×2 pixels in the cells' image, the top pair's then the bottom
// pair's: its base sprite (low, high), the shade it takes (render.Taking), its flags; its way's
// sprite, the way's Links, the covers on it (bit i the i-th); the way's width in 64ths (low,
// high), how far it has faded in 255ths, how far the cell's kind sways in 255ths; how high it
// stands in 64ths (low, high), its material and the red of the one laid over it in 255ths
const (
	blockSprite    = 0
	blockTakes     = 2
	blockFlags     = 3
	blockWaySprite = 4
	blockLinks     = 6
	blockCovers    = 7
	blockWidth     = 8
	blockFade      = 10
	blockSway      = 11
	blockHeight    = 12
	blockMaterial  = 14
	blockOverlay   = 15
	blockBytes     = 16
)

// a cell's flags: of itself, then of it or a neighbour, so a pixel far from any reads no more
const (
	flagShaded    = 1 // drawn by its material, no sprite
	flagWay       = 2 // a way runs across it
	flagSways     = 4 // what stands on it sways
	flagNearCover = 8
	flagNearSway  = 16
	flagOverlaid  = 32 // a material lies over its sprite
)

// sprites a row of the rectangles' image holds, two pixels each
const rectsPerRow = 256

// the vertices of an instance: a cell's covers' four quarters, two triangles each; a cell's way's
// eight bands, two triangles each, and its hub, an octagon fanned in six
const (
	coverVertices = 24
	wayVertices   = 66
)

// tiles lays a flat board's tiles on the GPU: what each cell shows is kept in an image, a block of
// 2×2 pixels a cell, the sheet's rectangles in another, and the passes find every pixel's cell
// through the camera's rays and sample the sheet there, the covers and the ways laid an instance
// a cell they lie on
type tiles struct {
	opts  render.DrawMeshOptions
	own   map[string]any
	cells *render.Image
	rects *render.Image
	// the sprites the rectangles' image holds, by SpriteID; what they were read from
	rected []bool
	sheet  render.AtlasSource
	vers   []uint64 // one more than each cell's version the cells' image holds, by ordinal
	at     uint64   // one more than the board's count of changes the image holds; 0 none
	block  [blockBytes]byte
	buf    []byte
	rect   [8]byte
	// each cell's flags and covers as the image holds them, by ordinal, and how many cells hold
	// what sways, what is drawn by its material and what has one laid over it
	flags, covered            []byte
	swaying, shaded, overlaid int
	covers                    []render.SpriteID
	// the instances of the cells a way runs across and of a cell and a cover lying on it or a
	// neighbour: column, row, cover, a vec4 each
	wayAt, coverAt []float32
	// the cells to write anew this frame: stamp where a cell is among them, by ordinal
	marks   []uint32
	stamp   uint32
	changed []int
	laid    bool // whether draw laid the tiles this frame, its uniforms set
}

// refresh brings the cells' image up to the board as l reads it, the sheet sheet: every cell
// anew at first, after the sheet changed and where many changed, else the changed ones alone
func (g *tiles) refresh(l *Renderer, sheet render.AtlasSource) {
	shape, ok := l.board.Shape()
	if !ok {
		return
	}
	cols, rows := int(shape.Cols), int(shape.Rows)
	n := cols * rows
	all := g.at == 0 || sheet != g.sheet || len(g.vers) != n
	if sheet != g.sheet {
		g.sheet, g.rects, g.rected = sheet, nil, g.rected[:0]
	}
	if g.cells == nil || g.cells.Bounds().Dx() != 2*cols || g.cells.Bounds().Dy() != 2*rows {
		g.cells, all = render.NewImage(2*cols, 2*rows), true
	}
	if len(g.vers) != n {
		g.vers, g.marks = make([]uint64, n), make([]uint32, n)
		g.flags, g.covered = make([]byte, n), make([]byte, n)
		g.swaying, g.shaded, g.overlaid, g.stamp = 0, 0, 0, 0
	}
	if !all && g.at == l.board.Changes()+1 {
		return
	}
	g.at = l.board.Changes() + 1
	if covers := laidOf(l).covers(); !slicesEqual(covers, g.covers) {
		g.covers, all = covers, true
		for _, id := range covers {
			g.rectFor(id)
		}
	}
	l.nextTops()
	defer g.instances(l, cols)
	if all {
		g.writeAll(l, cols, rows)
		return
	}
	// a cell changed and its neighbours, whose flags of what lies near may have
	if g.stamp++; g.stamp == 0 {
		clear(g.marks)
		g.stamp = 1
	}
	g.changed = g.changed[:0]
	mark := func(c cell.ID) {
		if i, ok := l.board.Ordinal(c); ok && g.marks[i] != g.stamp {
			g.marks[i] = g.stamp
			g.changed = append(g.changed, i)
		}
	}
	for i := range n {
		c := l.board.At(uint32(i%cols), uint32(i/cols))
		if v := l.board.CellVersion(c) + 1; v == g.vers[i] {
			continue
		}
		mark(c)
		for d := range l.board.Directions() {
			if nb, ok := l.board.Toward(c, d); ok {
				mark(nb)
			}
		}
		if len(g.changed) > n/8 {
			g.writeAll(l, cols, rows)
			return
		}
	}
	for _, i := range g.changed {
		x, y := i%cols, i/cols
		g.fill(l, l.board.At(uint32(x), uint32(y)), i)
		b := &g.block
		g.cells.SubImage(image.Rect(2*x, 2*y, 2*x+2, 2*y+2)).WritePixels(b[:])
	}
}

func (g *tiles) writeAll(l *Renderer, cols, rows int) {
	stride := 8 * cols
	if need := stride * 2 * rows; cap(g.buf) < need {
		g.buf = make([]byte, need)
	} else {
		g.buf = g.buf[:need]
	}
	for y := range rows {
		for x := range cols {
			i := y*cols + x
			g.fill(l, l.board.At(uint32(x), uint32(y)), i)
			at := 2*y*stride + 8*x
			copy(g.buf[at:at+8], g.block[:8])
			copy(g.buf[at+stride:at+stride+8], g.block[8:])
		}
	}
	g.cells.WritePixels(g.buf)
}

// fill lays what c, of ordinal i, shows into the block, its sprites into the rectangles
func (g *tiles) fill(l *Renderer, c cell.ID, i int) {
	g.vers[i] = l.board.CellVersion(c) + 1
	g.block = [blockBytes]byte{}
	b := &g.block
	kind := l.board.Kind(c)
	if m, shaded := l.shaded[kind.Name]; shaded {
		b[blockFlags] |= flagShaded
		b[blockMaterial] = byte(m)
	} else {
		base := l.topOf(c).sprite
		if l.dress != nil {
			base = l.dress.Base(c, base)
		}
		g.sprite(blockSprite, base)
		if o, ok := l.dress.(Overlaid); ok {
			if m, red, ok := o.Overlay(c); ok {
				b[blockFlags] |= flagOverlaid
				b[blockMaterial], b[blockOverlay] = byte(m), unit255(red)
			}
		}
	}
	b[blockTakes] = byte(takesOf(kind.Takes))
	laid := laidOf(l)
	if w := laid.way(c); w.Runs() {
		b[blockFlags] |= flagWay
		g.sprite(blockWaySprite, w.Kind.SpriteID)
		var links cell.Links // towards a neighbour there is
		for d := range l.board.Directions() {
			if _, ok := l.board.Toward(c, d); ok && w.Links.Has(d) {
				links |= 1 << d
			}
		}
		b[blockLinks] = byte(links)
		put16(b[blockWidth:], w.Width*64)
		b[blockFade] = unit255(w.Fade)
	}
	b[blockCovers] = laid.covered(c)
	g.covered[i] = b[blockCovers]
	if top := l.topOf(c); top.sway > 0 && top.height > 0 {
		b[blockFlags] |= flagSways
		b[blockSway] = unit255(top.sway)
		put16(b[blockHeight:], top.height*64)
	}
	b[blockFlags] |= g.near(l, laid, c)
	g.count(i, b[blockFlags])
}

// near is the flags of what lies on c or a neighbour of it: a cover, what sways
func (g *tiles) near(l *Renderer, laid laid, c cell.ID) byte {
	var f byte
	of := func(n cell.ID) {
		if laid.covered(n) != 0 {
			f |= flagNearCover
		}
		if top := l.topOf(n); top.sway > 0 && top.height > 0 {
			f |= flagNearSway
		}
	}
	of(c)
	for d := range l.board.Directions() {
		if nb, ok := l.board.Toward(c, d); ok {
			of(nb)
		}
	}
	return f
}

// count keeps the counts of the cells flagged as the image holds them, cell i now flagged f
func (g *tiles) count(i int, f byte) {
	was := g.flags[i]
	g.flags[i] = f
	for _, k := range [3]struct {
		bit byte
		n   *int
	}{{flagSways, &g.swaying}, {flagShaded, &g.shaded}, {flagOverlaid, &g.overlaid}} {
		switch {
		case was&k.bit == 0 && f&k.bit != 0:
			*k.n++
		case was&k.bit != 0 && f&k.bit == 0:
			*k.n--
		}
	}
}

// instances are the cells a way runs across and, for each cover, the cells it lies on or beside,
// in the order the cells are laid
func (g *tiles) instances(l *Renderer, cols int) {
	g.wayAt, g.coverAt = g.wayAt[:0], g.coverAt[:0]
	for i, f := range g.flags {
		x, y := float32(i%cols), float32(i/cols)
		if f&flagWay != 0 {
			g.wayAt = append(g.wayAt, x, y, 0, 0)
		}
		if f&flagNearCover == 0 {
			continue
		}
		bits := g.covered[i]
		l.eachRound(l.board.At(uint32(i%cols), uint32(i/cols)), func(n cell.ID, ok bool) {
			if k, on := l.board.Ordinal(n); ok && on {
				bits |= g.covered[k]
			}
		})
		for k := range g.covers {
			if bits&(1<<k) != 0 {
				g.coverAt = append(g.coverAt, x, y, float32(k), 0)
			}
		}
	}
}

func put16(b []byte, v float32) {
	u := uint16(min(max(v+0.5, 0), 65535))
	b[0], b[1] = byte(u), byte(u>>8)
}

func unit255(v float32) byte { return byte(min(max(v, 0), 1)*255 + 0.5) }

// laid is what a Laid Dressing says of the ways and the covers; none without one
type laid struct{ d Laid }

func laidOf(l *Renderer) laid {
	d, _ := l.dress.(Laid)
	return laid{d}
}

func (a laid) way(c cell.ID) cell.Way {
	if a.d == nil {
		return cell.Way{}
	}
	return a.d.Way(c)
}

func (a laid) covered(c cell.ID) byte {
	if a.d == nil {
		return 0
	}
	return a.d.Covered(c)
}

func (a laid) covers() []render.SpriteID {
	if a.d == nil {
		return nil
	}
	cs := a.d.Covers()
	return cs[:min(len(cs), 8)]
}

func slicesEqual(a, b []render.SpriteID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sprite lays id at the block's byte at, low then high, and its rectangle into the rectangles
func (g *tiles) sprite(at int, id render.SpriteID) {
	g.block[at], g.block[at+1] = byte(id), byte(id>>8)
	g.rectFor(id)
}

// rectFor lays the rectangle of id into the rectangles, unless they hold it
func (g *tiles) rectFor(id render.SpriteID) {
	k := int(id)
	if k < len(g.rected) && g.rected[k] {
		return
	}
	rows := k/rectsPerRow + 1
	if g.rects == nil || g.rects.Bounds().Dy() < rows {
		g.grow(rows)
	}
	for len(g.rected) <= k {
		g.rected = append(g.rected, false)
	}
	g.rected[k] = true
	g.writeRect(k)
}

// grow makes the rectangles' image rows high, the sprites it held laid again
func (g *tiles) grow(rows int) {
	g.rects = render.NewImage(2*rectsPerRow, max(rows, 2*g.rowsOf()))
	for k, done := range g.rected {
		if done {
			g.writeRect(k)
		}
	}
}

func (g *tiles) rowsOf() int {
	if g.rects == nil {
		return 1
	}
	return g.rects.Bounds().Dy()
}

func (g *tiles) writeRect(k int) {
	x0, y0, x1, y1 := g.sheet.UV(render.SpriteID(k))
	r := &g.rect
	for j, v := range [4]float32{x0, y0, x1, y1} {
		u := uint16(v)
		r[2*j], r[2*j+1] = byte(u), byte(u>>8)
	}
	x, y := 2*(k%rectsPerRow), k/rectsPerRow
	g.rects.SubImage(image.Rect(x, y, x+2, y+1)).WritePixels(r[:])
}

// seeing is how the tiles are seen: the lines of sight across a viewport w by h, the world's copies
// from the x0-th to the x1-th across and the y0-th to the y1-th down, and whether they sway
type seeing struct {
	rays           camera.RayField
	w, h           float32
	x0, x1, y0, y1 int
	sways          bool
}

// draw lays the tiles as v sees them into t's screen, lit as lit says, in tint, of the sectors
// seen at their levels; their ground's depth into t's depth where it has one
func (g *tiles) draw(t render.Target, v seeing, u render.Uniforms, l *Renderer, tint render.Light,
	lit bool, seen *seenSectors) {
	g.laid = false
	if g.cells == nil || g.sheet == nil {
		return
	}
	f := v.rays
	if g.own == nil {
		g.own, g.opts.Uniforms = map[string]any{}, map[string]any{}
	}
	u.Into(g.opts.Uniforms)
	at := t.Screen.Bounds().Min
	g.set("ViewAt", float32(at.X), float32(at.Y))
	g.set("RayAt", f.Origin[:]...)
	g.set("RayDX", f.DX[:]...)
	g.set("RayDY", f.DY[:]...)
	g.set("RayDir", f.Dir[:]...)
	g.set("RayDDX", f.DDX[:]...)
	g.set("RayDDY", f.DDY[:]...)
	tr, placed := camera.SceneTransform(f, v.w, v.h)
	g.set("TileViewProj", tr.M[:]...)
	deep := t.Depth != nil && placed
	g.set("TileDeep", flag(deep))
	shape, _ := l.board.Shape()
	g.set("TileHex", flag(shape.Hex))
	g.set("TileBox", float32(l.cellW), float32(l.cellH))
	g.set("TileSide", float32(shape.Cell))
	g.set("TileCells", float32(shape.Cols), float32(shape.Rows))
	g.set("TileWrap", flag(l.space.Edges.WrapsX()), flag(l.space.Edges.WrapsY()))
	g.set("TileLit", flag(lit))
	g.set("TileTint", tint[:]...)
	sways := v.sways && g.swaying > 0
	full := shape.Hex || sways || g.overlaid > 0
	g.set("TileSways", flag(sways))
	g.set("TileFull", flag(full))
	var covers [8]float32
	for i, id := range g.covers {
		covers[i] = float32(id)
	}
	g.set("TileCovers", covers[:4]...)
	g.set("TileCoversOn", covers[4:]...)
	g.set("TileShift", 0, 0)
	g.laid = true
	g.opts.Images = [4]*render.Image{g.sheet.Atlas(), g.rects, g.cells,
		seen.uniforms(g.set, "Tile")}
	// pixel by pixel, the ground's depth with it
	g.opts.Vertices, g.opts.Instances = 3, nil
	g.opts.Depth, g.opts.WriteDepth = nil, false
	if deep {
		g.opts.Depth, g.opts.WriteDepth = t.Depth, true
	}
	if !shape.Hex {
		t.Screen.DrawMesh(nil, plainShader, &g.opts)
	}
	if full {
		t.Screen.DrawMesh(nil, fullShader, &g.opts)
	}
	// over them the covers, then the ways, a copy across each seam the view takes in
	g.opts.Depth, g.opts.WriteDepth = nil, false
	if placed && (len(g.coverAt) > 0 || len(g.wayAt) > 0) {
		for y := v.y0; y <= v.y1; y++ {
			for x := v.x0; x <= v.x1; x++ {
				g.set("TileShift", float32(x)*float32(l.space.Width),
					float32(y)*float32(l.space.Height))
				if !shape.Hex {
					g.instanced(t, coversShader, g.coverAt, coverVertices)
				}
				g.instanced(t, waysShader, g.wayAt, wayVertices)
			}
		}
		g.set("TileShift", 0, 0)
	}
}

// materials lays the cells drawn by their material over what draw laid, with its uniforms
func (g *tiles) materials(t render.Target) {
	if g.shaded == 0 || !g.laid {
		return
	}
	g.opts.Vertices, g.opts.Instances = 3, nil
	g.opts.Depth, g.opts.WriteDepth = nil, false
	t.Screen.DrawMesh(nil, materialShader, &g.opts)
}

func (g *tiles) instanced(t render.Target, s *render.Shader, at []float32, vertices int) {
	if len(at) == 0 {
		return
	}
	g.opts.Instances, g.opts.Vertices = at, vertices
	t.Screen.DrawMesh(nil, s, &g.opts)
}

// copies is the world's copies a camera from above sees, a world apart along an axis that wraps:
// from the x0-th to the x1-th across, y0-th to y1-th down; only the world itself through another
func (l *Renderer) copies(cam camera.Camera) (x0, x1, y0, y1 int) {
	if _, top := cam.Projection().(camera.TopDown); !top {
		return 0, 0, 0, 0
	}
	w, h := cam.Viewport()
	left, top := cam.FromScreen(0, 0)
	right, bottom := cam.FromScreen(w, h)
	along := func(lo, hi float32, size uint32, wraps bool) (int, int) {
		if !wraps || size == 0 {
			return 0, 0
		}
		s := float64(size)
		return int(math.Floor(float64(lo) / s)), int(math.Floor(float64(hi) / s))
	}
	x0, x1 = along(left, right, l.space.Width, l.space.Edges.WrapsX())
	y0, y1 = along(top, bottom, l.space.Height, l.space.Edges.WrapsY())
	return x0, x1, y0, y1
}

func (g *tiles) set(name string, v ...float32) {
	u, ok := g.own[name]
	if s, _ := u.([]float32); !ok || len(s) != len(v) {
		u = make([]float32, len(v))
		g.own[name] = u
	}
	copy(u.([]float32), v)
	g.opts.Uniforms[name] = u
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
