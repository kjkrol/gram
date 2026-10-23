package painter

import (
	"image"

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

// the vertices of a way's instance: six a piece
const wayVertices = 6 * lanePieces

// the largest number of pieces a way lays: a stem through eight, six joining it, all running
// into water (paint_ways.wgsl)
const lanePieces = 46

// baking is what the bake's passes keep from one bake to the next
type baking struct {
	opts  render.DrawMeshOptions
	own   map[string]any
	at    []float32 // the cells to paint anew: column, row, a vec4 each
	ways  []float32 // the cells whose ways reach them, row by row
	marks []uint32
	stamp uint32
	mask  *render.Image // the cells to paint anew, white
	pix   []byte
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
// and the water's layers where s has them, each the cell's ground, then the ways across it
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
		l.paintSet(dst, layer, origin, float32(px), float32(px)/float32(l.sq.Cell), 1, !whole)
		b := &l.baking
		b.opts.Images = [4]*render.Image{atlas.Atlas(), l.cells.rects, l.cells.img, b.mask}
		b.opts.Instances, b.opts.Vertices = at, 6
		dst.DrawMesh(nil, groundShader, &b.opts)
		if len(b.ways) > 0 {
			b.opts.Instances, b.opts.Vertices = b.ways, wayVertices
			dst.DrawMesh(nil, waysShader, &b.opts)
		}
	}
	layer(s.img, layerAlbedo, image.Pt(0, s.top), s.px)
	if s.kinds != nil {
		layer(s.kinds, layerKinds, image.Point{}, s.px)
	}
	if s.water != nil {
		for q := range water.Layers {
			layer(s.water, layerWater+q, s.waterLayer(q).Min, s.wpx)
		}
	}
}

// wayCells are the cells whose ways may reach the cells at, row by row: those within one of them
func (l *Painter) wayCells(at []float32, whole bool) {
	cols, rows := int(l.sq.Cols), int(l.sq.Rows)
	b := &l.baking
	b.ways = b.ways[:0]
	for i := range cols * rows {
		x, y := i%cols, i/cols
		c, _ := l.cellAt(int64(x), int64(y))
		if !l.topOf(c).way.Runs() || !whole && !l.nearMask(x, y) {
			continue
		}
		b.ways = append(b.ways, float32(x), float32(y), 0, 0)
	}
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
	b.set("PaintCell", float32(shape.Cell))
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
