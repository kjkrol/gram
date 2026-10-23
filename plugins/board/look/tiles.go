package look

import (
	"image"

	"github.com/kjkrol/gram/camera"
	_ "github.com/kjkrol/gram/plugins/atmosphere/air" // the Wind the shader leans the tiles in
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

var tilesShader = render.NewMeshShaderWith(
	"board tiles",
	render.Files(shaders, "shaders/tiles.wgsl"),
	[]render.Uniform{
		{Name: "ViewAt", Size: 2},
		{Name: "RayAt", Size: 3},
		{Name: "RayDX", Size: 3},
		{Name: "RayDY", Size: 3},
		{Name: "RayDir", Size: 3},
		{Name: "RayDDX", Size: 3},
		{Name: "RayDDY", Size: 3},
		{Name: "TileViewProj", Size: 16},
		{Name: "TileDeep", Size: 1},
		{Name: "TileHex", Size: 1},
		{Name: "TileBox", Size: 2},
		{Name: "TileSide", Size: 1},
		{Name: "TileCells", Size: 2},
		{Name: "TileWrap", Size: 2},
		{Name: "TileLit", Size: 1},
		{Name: "TileTint", Size: 3},
		{Name: "TileWays", Size: 1},
		{Name: "TileSways", Size: 1},
		{Name: "TileShaded", Size: 1},
		{Name: "TileCoverCount", Size: 1},
		{Name: "TileCovers", Size: 4},
		{Name: "TileCoversOn", Size: 4},
	},
)

// the bytes of a cell's block of 2×2 pixels in the cells' image, the top pair's then the bottom
// pair's: its base sprite (low, high), the shade it takes (render.Taking), its flags; its way's
// sprite, the way's Links, the covers on it (bit i the i-th); the way's width in 64ths (low,
// high), how far it has faded in 255ths, how far the cell's kind sways in 255ths; how high it
// stands in 64ths (low, high) and its material
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
	blockBytes     = 16
)

// a cell's flags
const (
	flagShaded = 1 // drawn by its material, no sprite
	flagWay    = 2 // a way runs across it
	flagSways  = 4 // what stands on it sways
)

// sprites a row of the rectangles' image holds, two pixels each
const rectsPerRow = 256

// tiles lays a flat board's tiles on the GPU: what each cell shows is kept in an image, a block of
// 2×2 pixels a cell, the sheet's rectangles in another, and a shader finds every pixel's cell
// through the camera's rays and samples the sheet there
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
	// each cell's flags as the image holds them, by ordinal, and how many cells hold each
	flags                 []byte
	ways, swaying, shaded int
	covers                []render.SpriteID
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
		g.vers, g.flags = make([]uint64, n), make([]byte, n)
		g.ways, g.swaying, g.shaded = 0, 0, 0
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
	if all {
		g.writeAll(l, cols, rows)
		return
	}
	var changed []int
	for i := range n {
		c := l.board.At(uint32(i%cols), uint32(i/cols))
		if v := l.board.CellVersion(c) + 1; v != g.vers[i] {
			changed = append(changed, i)
			if len(changed) > n/8 {
				g.writeAll(l, cols, rows)
				return
			}
		}
	}
	for _, i := range changed {
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
		l.place(&l.tile, c)
		g.sprite(blockSprite, l.tile.Base())
	}
	b[blockTakes] = byte(takesOf(kind.Takes))
	laid := laidOf(l)
	if w := laid.way(c); w.Runs() {
		b[blockFlags] |= flagWay
		g.sprite(blockWaySprite, w.Kind.SpriteID)
		b[blockLinks] = byte(w.Links)
		put16(b[blockWidth:], w.Width*64)
		b[blockFade] = unit255(w.Fade)
	}
	b[blockCovers] = laid.covered(c)
	if top := l.topOf(c); top.sway > 0 && top.height > 0 {
		b[blockFlags] |= flagSways
		b[blockSway] = unit255(top.sway)
		put16(b[blockHeight:], top.height*64)
	}
	g.count(i, b[blockFlags])
}

// count keeps the counts of the cells flagged as the image holds them, cell i now flagged f
func (g *tiles) count(i int, f byte) {
	was := g.flags[i]
	g.flags[i] = f
	for _, k := range []struct {
		bit byte
		n   *int
	}{{flagWay, &g.ways}, {flagSways, &g.swaying}, {flagShaded, &g.shaded}} {
		switch {
		case was&k.bit == 0 && f&k.bit != 0:
			*k.n++
		case was&k.bit != 0 && f&k.bit == 0:
			*k.n--
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
	d, _ := l.tile.dress.(Laid)
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

// draw lays the tiles through cam into t's screen, lit as lit says, in tint; their ground's depth
// into t's depth where it has one
func (g *tiles) draw(t render.Target, cam camera.Camera, u render.Uniforms, l *Renderer,
	tint render.Light, lit bool) {
	if g.cells == nil || g.sheet == nil {
		return
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	f, ok := rays.Rays()
	if !ok {
		return
	}
	if g.own == nil {
		g.own, g.opts.Uniforms = map[string]any{}, map[string]any{}
		g.opts.Vertices = 3
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
	deep := false
	if t.Depth != nil {
		w, h := cam.Viewport()
		if tr, ok := camera.SceneTransform(f, w, h); ok {
			g.set("TileViewProj", tr.M[:]...)
			deep = true
		}
	}
	g.opts.Depth, g.opts.WriteDepth = nil, false
	if deep {
		g.opts.Depth, g.opts.WriteDepth = t.Depth, true
	}
	g.set("TileDeep", flag(deep))
	shape, _ := l.board.Shape()
	g.set("TileHex", flag(shape.Hex))
	g.set("TileBox", float32(l.cellW), float32(l.cellH))
	g.set("TileSide", float32(shape.Cell))
	g.set("TileCells", float32(shape.Cols), float32(shape.Rows))
	g.set("TileWrap", flag(l.space.Edges.WrapsX()), flag(l.space.Edges.WrapsY()))
	g.set("TileLit", flag(lit))
	g.set("TileTint", tint[:]...)
	g.set("TileWays", flag(g.ways > 0))
	_, sways := l.mapping.Look().(Swaying)
	g.set("TileSways", flag(sways && g.swaying > 0))
	g.set("TileShaded", flag(g.shaded > 0))
	var covers [8]float32
	for i, id := range g.covers {
		covers[i] = float32(id)
	}
	g.set("TileCoverCount", float32(len(g.covers)))
	g.set("TileCovers", covers[:4]...)
	g.set("TileCoversOn", covers[4:]...)
	g.opts.Images = [4]*render.Image{g.sheet.Atlas(), g.rects, g.cells}
	t.Screen.DrawMesh(nil, tilesShader, &g.opts)
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
