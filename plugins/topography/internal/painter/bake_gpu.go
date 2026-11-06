package painter

import (
	"image"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/topography/internal/water"
	"github.com/kjkrol/gram/render"
)

// the bake's passes over the cells to paint anew and the ways across them, an instance a cell
var (
	groundShader = render.NewMeshShaderWith("painter's ground", render.Files(shaders,
		"shaders/paint_common.wgsl", "shaders/paint_ground.wgsl"), paintUniforms).Instanced(1)
	waysShader = render.NewMeshShaderWith("painter's ways", render.Files(shaders,
		"shaders/paint_common.wgsl", "shaders/paint_ways.wgsl", "shaders/ways_bake.wgsl"),
		paintUniforms).Instanced(1)
	topsShader = render.NewMeshShaderWith("painter's ways over the tops", render.Files(shaders,
		"shaders/paint_common.wgsl", "shaders/paint_ways.wgsl", "shaders/ways_tops.wgsl"),
		paintUniforms).Instanced(1)
)

var paintUniforms = []render.Uniform{
	{Name: "PaintCells", Size: 2}, {Name: "PaintWrap", Size: 2}, {Name: "PaintPx", Size: 1},
	{Name: "PaintTarget", Size: 2}, {Name: "PaintOrigin", Size: 2}, {Name: "PaintLayer", Size: 1},
	{Name: "PaintHex", Size: 1}, {Name: "PaintCell", Size: 1}, {Name: "PaintLow", Size: 1},
	{Name: "PaintSpan", Size: 1}, {Name: "PaintFine", Size: 1}, {Name: "PaintDetail", Size: 1},
	{Name: "PaintHeights", Size: 1}, {Name: "PaintScale", Size: 1}, {Name: "PaintMasked", Size: 1},
	{Name: "PaintFlowSpan", Size: 1}, {Name: "PaintCoast", Size: 1}, {Name: "LaneGlint", Size: 1},
	{Name: "LaneStream", Size: 1},
}

// the layers the bake paints (paint_common.wgsl): the albedo, the kinds' details, then the
// water's from water.FlowLayer on
const (
	layerAlbedo = 0
	layerKinds  = 1
	layerWater  = 2
)

// baking is what the bake's passes keep from one bake to the next
type baking struct {
	opts  render.DrawMeshOptions
	own   map[string]any
	at    []float32 // the cells to paint anew: column, row, a vec4 each
	ways  []float32 // the cells whose ways reach them, row by row
	most  int       // the most pieces one of the ways lays
	marks []uint32
	stamp uint32
	mask  *render.Image // the cells to paint anew, white
	pix   []byte
	rects []image.Rectangle // the albedo's pixels to shrink anew
}

// toPaint are the cells to paint anew after the cells changed as the last refresh says: every
// cell within two of a changed one, whose blends or ways may have; all of them where whole says
// so, the cells were laid whole or over an eighth would be
func (l *Painter) toPaint(whole bool) []float32 {
	g, b := &l.cells, &l.baking
	cols, rows := int(l.sq.Cols), int(l.sq.Rows)
	n := cols * rows
	b.at = b.at[:0]
	all := func() []float32 {
		b.at = b.at[:0]
		for i := range n {
			b.at = append(b.at, float32(i%cols), float32(i/cols), 0, 0)
		}
		return b.at
	}
	if whole || g.changed == nil {
		return all()
	}
	if len(b.marks) != n {
		b.marks, b.stamp = make([]uint32, n), 0
	}
	if b.stamp++; b.stamp == 0 {
		clear(b.marks)
		b.stamp = 1
	}
	for _, i := range g.changed {
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				c, ok := l.cellAt(int64(i%cols+dx), int64(i/cols+dy))
				if !ok {
					continue
				}
				j, _ := l.ordinal(c)
				if b.marks[j] != b.stamp {
					b.marks[j] = b.stamp
					b.at = append(b.at, float32(j%cols), float32(j/cols), 0, 0)
				}
			}
		}
	}
	if len(b.at)/4 > n/8 {
		return all()
	}
	return b.at
}

// bake paints the cells at anew into the sheet s out of atlas: the albedo, the kinds' details
// where some kind has one and the water's layers where s has them, each the cell's ground, then
// the ways across it; the mouths' layer the ways alone
func (l *Painter) bake(s *groundSheet, atlas render.AtlasSource, at []float32) {
	if len(at) == 0 {
		return
	}
	cols, rows := int(l.sq.Cols), int(l.sq.Rows)
	whole := len(at) == 4*cols*rows
	if !whole {
		l.baking.paintMask(at, cols, rows)
	}
	l.wayCells(at, whole)
	layer := func(dst *render.Image, layer int, origin image.Point, px int) {
		if whole {
			dst.SubImage(image.Rect(origin.X, origin.Y, origin.X+cols*px,
				origin.Y+rows*px)).Clear()
		} else {
			for k := 0; k < len(at); k += 4 {
				x, y := origin.X+int(at[k])*px, origin.Y+int(at[k+1])*px
				dst.SubImage(image.Rect(x, y, x+px, y+px)).Clear()
			}
		}
		l.paintSet(dst, layer, origin, float32(px), float32(px)/l.sq.Cell, 1, !whole)
		b := &l.baking
		b.opts.Images = [4]*render.Image{atlas.Atlas(), l.cells.rects, l.cells.img, b.mask}
		if layer != layerWater+water.MouthLayer {
			b.opts.Instances, b.opts.Vertices = at, 6
			dst.DrawMesh(nil, groundShader, &b.opts)
		}
		if len(b.ways) > 0 {
			b.opts.Instances, b.opts.Vertices = b.ways, 6*b.most
			dst.DrawMesh(nil, waysShader, &b.opts)
		}
	}
	layer(s.img, layerAlbedo, image.Pt(0, s.top), s.px)
	if sheet, _ := l.Details(); s.kinds != nil && sheet != nil {
		layer(s.kinds, layerKinds, image.Point{}, s.px)
	}
	if s.water != nil {
		for q := range water.Layers {
			layer(s.water, layerWater+q, s.waterLayer(q).Min, s.wpx)
		}
	}
	l.shrink(s, at, whole)
}

// shrink lays the albedo's pixels under the cells at anew into its half, and the half's under
// them into its quarter; all of them where whole
func (l *Painter) shrink(s *groundSheet, at []float32, whole bool) {
	b := &l.baking
	b.rects = b.rects[:0]
	if whole {
		b.rects = append(b.rects, s.img.Bounds())
	} else {
		b.rects = cellRects(at, s.px, s.top, s.img.Bounds(), b.rects)
	}
	for k, r := range b.rects {
		b.rects[k] = halve(s.half, s.img, r)
	}
	for _, r := range b.rects {
		halve(s.quarter, s.half, r)
	}
}

// cellRects are the pixels of the cells at, px a cell from the row top down: a run of them along
// a row a rectangle, joined to the one above where it spans the same cells, widened out to fours
// of pixels, so its half and quarter stand on whole pixels, and held within bounds
func cellRects(at []float32, px, top int, bounds image.Rectangle,
	dst []image.Rectangle) []image.Rectangle {
	from := len(dst)
	for k := 0; k < len(at); {
		x, y := int(at[k]), int(at[k+1])
		end := x + 1
		for k += 4; k < len(at) && int(at[k+1]) == y && int(at[k]) == end; k += 4 {
			end++
		}
		if n := len(dst); n > from && dst[n-1].Min.X == x && dst[n-1].Max.X == end &&
			dst[n-1].Max.Y == y {
			dst[n-1].Max.Y++
			continue
		}
		dst = append(dst, image.Rect(x, y, end, y+1))
	}
	for k := from; k < len(dst); k++ {
		c := dst[k]
		r := image.Rect(c.Min.X*px&^3, (top+c.Min.Y*px)&^3, (c.Max.X*px+3)&^3,
			(top+c.Max.Y*px+3)&^3)
		dst[k] = r.Intersect(bounds)
	}
	return dst
}

// halve lays the pixels r of src into dst at half their size, each the mean of the 2 by 2 under
// it, r's corner on even pixels; it is the pixels of dst laid
func halve(dst, src *render.Image, r image.Rectangle) image.Rectangle {
	to := dst.SubImage(image.Rect(r.Min.X/2, r.Min.Y/2, (r.Max.X+1)/2, (r.Max.Y+1)/2))
	to.Clear()
	var op render.DrawImageOptions
	op.GeoM.Scale(0.5, 0.5)
	op.GeoM.Translate(float64(to.Bounds().Min.X), float64(to.Bounds().Min.Y))
	op.Filter = render.FilterLinear
	to.DrawImage(src.SubImage(r), &op)
	return to.Bounds()
}

// wayCells are the cells whose ways may reach the cells at, row by row: those within one of them
func (l *Painter) wayCells(at []float32, whole bool) {
	cols, rows := int(l.sq.Cols), int(l.sq.Rows)
	b := &l.baking
	b.ways, b.most = b.ways[:0], 0
	for i := range cols * rows {
		x, y := i%cols, i/cols
		c, _ := l.cellAt(int64(x), int64(y))
		w := l.topOf(c).way
		if !w.Runs() || !whole && !l.nearMask(x, y) {
			continue
		}
		b.ways = append(b.ways, float32(x), float32(y), 0, 0)
		b.most = max(b.most, l.piecesOf(c, w))
	}
}

// piecesOf is how many pieces the way w across c lays at most (paint_ways.wgsl, pieceOf): alone
// one; out to one neighbour its end and the band out; to more the stem through two and a band
// joining it for each of the rest; and one out to a cell lying under the rest running on under
// it, each in as many steps as U.PaintFine has
func (l *Painter) piecesOf(c cell.ID, w lane) int {
	steps := func(n int) int {
		if l.cellPx >= nearCell || l.square {
			return n
		}
		return max(n/3, 1)
	}
	outs, under := 0, 0
	for d := range l.board.Directions() {
		if w.Links&(1<<d) == 0 {
			continue
		}
		outs++
		if n, ok := l.board.Toward(c, d); ok {
			if t := l.topOf(n); !t.way.Runs() && t.under {
				under++
			}
		}
	}
	switch outs {
	case 0:
		return 1
	case 1:
		return 1 + steps(4) + steps(2)*under
	}
	return steps(6) + steps(4)*(outs-2) + steps(2)*under
}

// paintMask marks the cells at in the mask, white, the rest clear
func (b *baking) paintMask(at []float32, cols, rows int) {
	if b.mask == nil || b.mask.Bounds().Dx() != cols || b.mask.Bounds().Dy() != rows {
		b.mask = render.NewImage(cols, rows)
	}
	if len(b.pix) != 4*cols*rows {
		b.pix = make([]byte, 4*cols*rows)
	}
	clear(b.pix)
	for k := 0; k < len(at); k += 4 {
		i := 4 * (int(at[k+1])*cols + int(at[k]))
		b.pix[i], b.pix[i+3] = 255, 255
	}
	b.mask.WritePixels(b.pix)
}

// nearMask reports whether the cell x, y or one round it is in the mask: whose ways may reach it
func (l *Painter) nearMask(x, y int) bool {
	b, cols := &l.baking, int(l.sq.Cols)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			c, ok := l.cellAt(int64(x+dx), int64(y+dy))
			if !ok {
				continue
			}
			j, _ := l.ordinal(c)
			if b.pix[4*((j/cols)*cols+j%cols)] != 0 {
				return true
			}
		}
	}
	return false
}

// paintSet sets the passes' uniforms for the layer of a target dst, px pixels a cell from origin
// and scale a world unit, the ways in their fine steps where fine, their water's shine times
// detail, only the cells marked painted where masked
func (l *Painter) paintSet(dst *render.Image, layer int, origin image.Point, px, scale,
	detail float32, masked bool) {
	b := &l.baking
	if b.own == nil {
		b.own, b.opts.Uniforms = map[string]any{}, map[string]any{}
	}
	t := dst.Bounds()
	shape, _ := l.board.Shape()
	b.set("PaintCells", float32(l.sq.Cols), float32(l.sq.Rows))
	b.set("PaintWrap", flag(l.sq.WrapX), flag(l.sq.WrapY))
	b.set("PaintPx", px)
	b.set("PaintTarget", float32(t.Dx()), float32(t.Dy()))
	b.set("PaintOrigin", float32(origin.X), float32(origin.Y))
	b.set("PaintLayer", float32(layer))
	b.set("PaintHex", flag(shape.Hex))
	b.set("PaintCell", shape.Cell)
	b.set("PaintLow", l.cells.low)
	b.set("PaintSpan", l.cells.span)
	b.set("PaintFine", flag(l.cellPx >= nearCell || l.square))
	b.set("PaintDetail", detail)
	b.set("PaintHeights", flag(l.heights))
	b.set("PaintScale", scale)
	b.set("PaintMasked", flag(masked))
	b.set("PaintFlowSpan", water.FlowSpan)
	b.set("PaintCoast", shoreReach)
	b.set("LaneGlint", float32(water.SeaGlint()))
	b.set("LaneStream", float32(water.RunningWater()))
}

func (b *baking) set(name string, v ...float32) {
	u, ok := b.own[name]
	if s, _ := u.([]float32); !ok || len(s) != len(v) {
		u = make([]float32, len(v))
		b.own[name] = u
	}
	copy(u.([]float32), v)
	b.opts.Uniforms[name] = u
}

func flag(on bool) float32 {
	if on {
		return 1
	}
	return 0
}
