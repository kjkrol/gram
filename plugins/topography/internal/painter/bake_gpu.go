package painter

import (
	"image"

	"github.com/kjkrol/gram/render"
)

// the bake's passes over the cells to paint anew, an instance a cell
var albedoShader = render.NewMeshShaderWith("painter's albedo", render.Files(shaders,
	"shaders/paint_common.wgsl", "shaders/paint_albedo.wgsl"), paintUniforms).Instanced(1)

var paintUniforms = []render.Uniform{
	{Name: "PaintCells", Size: 2}, {Name: "PaintWrap", Size: 2}, {Name: "PaintPx", Size: 1},
	{Name: "PaintTarget", Size: 2},
}

// baking is what the bake's passes keep from one bake to the next
type baking struct {
	opts  render.DrawMeshOptions
	own   map[string]any
	at    []float32 // the cells to paint anew: column, row, a vec4 each
	marks []uint32
	stamp uint32
}

// toPaint are the cells to paint anew after the cells changed as the last refresh says: every
// cell within two of a changed one, whose blends or ways may have; all of them where the cells
// were laid whole or over an eighth would be
func (l *Painter) toPaint() []float32 {
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
	if g.changed == nil {
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

// bakeAlbedo paints the cells at anew into dst, px pixels a cell, out of atlas: each one's base and
// the grounds round it running in
func (l *Painter) bakeAlbedo(dst *render.Image, atlas render.AtlasSource, px int, at []float32) {
	if len(at) == 0 {
		return
	}
	cols, rows := int(l.sq.Cols), int(l.sq.Rows)
	if len(at) == 4*cols*rows {
		dst.Clear()
	} else {
		for k := 0; k < len(at); k += 4 {
			x, y := int(at[k])*px, int(at[k+1])*px
			dst.SubImage(image.Rect(x, y, x+px, y+px)).Clear()
		}
	}
	b := &l.baking
	l.paintSet(dst, px)
	b.opts.Images = [4]*render.Image{atlas.Atlas(), l.cells.rects, l.cells.img, nil}
	b.opts.Instances, b.opts.Vertices = at, 6
	dst.DrawMesh(nil, albedoShader, &b.opts)
}

// paintSet sets the passes' uniforms for a target dst, px pixels a cell
func (l *Painter) paintSet(dst *render.Image, px int) {
	b := &l.baking
	if b.own == nil {
		b.own, b.opts.Uniforms = map[string]any{}, map[string]any{}
	}
	t := dst.Bounds()
	b.set("PaintCells", float32(l.sq.Cols), float32(l.sq.Rows))
	b.set("PaintWrap", flag(l.sq.WrapX), flag(l.sq.WrapY))
	b.set("PaintPx", float32(px))
	b.set("PaintTarget", float32(t.Dx()), float32(t.Dy()))
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
