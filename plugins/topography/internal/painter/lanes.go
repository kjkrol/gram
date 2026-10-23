package painter

import (
	"embed"
	"image"

	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// lanes is what DrawWays knows of the ways: whether water may run or glint on some
type lanes struct{ moving bool }

// DrawWays draws the ways across the tiles from above into dst, px pixels a world unit, out of
// atlas, under the frame's uniforms u, as Above readied the Painter: each way laid in pieces on
// the GPU as the bake lays them, its sprite blended and turned into its mix, the water running
// and glinting over it, row by row.
func (l *Painter) DrawWays(dst *render.Image, atlas render.AtlasSource, px float32,
	u render.Uniforms) {
	if px <= 0 {
		return
	}
	l.refresh(atlas)
	b, g := &l.baking, &l.lanes
	b.ways, g.moving = b.ways[:0], false
	for i := range l.board.CellCount() {
		x, y := uint32(i)%l.sq.Cols, uint32(i)/l.sq.Cols
		w := l.topOf(l.board.At(x, y)).way
		if !w.Runs() {
			continue
		}
		b.ways = append(b.ways, float32(x), float32(y), 0, 0)
		g.moving = g.moving || l.heights && (w.shine > 0 || w.mixes && w.mixShine > 0)
	}
	if len(b.ways) == 0 {
		return
	}
	if b.own == nil {
		b.own, b.opts.Uniforms = map[string]any{}, map[string]any{}
	}
	u.Into(b.opts.Uniforms)
	detail := min(max((l.cellPx-farCell)/(nearCell-farCell), 0), 1)
	l.paintSet(dst, layerAlbedo, image.Point{}, 0, px, detail, false)
	b.opts.Images = [4]*render.Image{atlas.Atlas(), l.cells.rects, l.cells.img, nil}
	b.opts.Instances, b.opts.Vertices = b.ways, wayVertices
	dst.DrawMesh(nil, topsShader, &b.opts)
}

// WaysMoving reports whether what DrawWays draws changes with the clock: water that may run or
// glint on some way.
func (l *Painter) WaysMoving() bool { return l.lanes.moving }
