package painter

import (
	"image"

	"github.com/kjkrol/gram/render"
)

// cells from a shore within which its waves turn to face it
const shoreReach = 3

// shoreMap is the way to the shore from every corner as Shores works it out: the cells that
// shone when it last did, a count of the board's changes seen and of its own
type shoreMap struct {
	shining []bool
	seen    uint64
	version uint64
	img     *render.Image
}

var coastShader = render.NewMeshShaderWith("painter's coast", render.Files(shaders,
	"shaders/paint_common.wgsl", "shaders/paint_coast.wgsl"), paintUniforms)

// Shores is the way to the shore from every corner of the square grid's lattice, a pixel a
// corner as the terrain's lattice holds it ((way + 1) / 2 in red and green, the distance over
// the reach in blue), how far off a shore is seen, and a count of the changes; worked out on the
// GPU anew as a cell comes to shine or stops. It is nil off a square grid.
func (l *Painter) Shores(atlas render.AtlasSource) (*render.Image, float32, uint64) {
	if !l.square {
		return nil, 0, 0
	}
	c := &l.shoreMap
	reach := shoreReach * l.sq.Cell
	if c.seen == l.board.Changes()+1 && c.img != nil {
		return c.img, reach, c.version
	}
	c.seen = l.board.Changes() + 1
	l.tables()
	n := int(l.sq.Cols) * int(l.sq.Rows)
	changed := len(c.shining) != n
	if changed {
		c.shining = make([]bool, n)
	}
	for i := range n {
		cell, _ := l.cellAt(int64(i%int(l.sq.Cols)), int64(i/int(l.sq.Cols)))
		if now := l.topOf(cell).shine > 0; now != c.shining[i] {
			c.shining[i], changed = now, true
		}
	}
	cols, rows, _, _, _ := l.ground().Lattice()
	if c.img == nil || c.img.Bounds().Dx() != cols || c.img.Bounds().Dy() != rows {
		c.img, changed = render.NewImage(cols, rows), true
	}
	if !changed {
		return c.img, reach, c.version
	}
	c.version++
	l.refresh(atlas)
	b := &l.baking
	l.paintSet(c.img, layerAlbedo, image.Point{}, 0, 0, 1, false)
	b.opts.Images = [4]*render.Image{nil, nil, l.cells.img, nil}
	b.opts.Instances, b.opts.Vertices = nil, 3
	c.img.DrawMesh(nil, coastShader, &b.opts)
	return c.img, reach, c.version
}
