package painter

import (
	"image"
	"image/color"

	"github.com/kjkrol/gram/plugins/topography/internal/water"
	"github.com/kjkrol/gram/render"
)

// whiteSheet is a sheet of white alone, for what is painted in the vertices' colours: every
// sprite of it is its white.
type whiteSheet struct{ img *render.Image }

var white = &whiteSheet{}

func (w *whiteSheet) Atlas() *render.Image {
	if w.img == nil {
		w.img = render.NewImage(3, 3)
		w.img.Fill(color.White)
	}
	return w.img
}
func (*whiteSheet) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 3, 3 }
func (*whiteSheet) White() (u, v float32)                           { return 1.5, 1.5 }

// waterLayer is the part of s's water layer q lies on.
func (s *groundSheet) waterLayer(q int) image.Rectangle {
	w, h := s.water.Bounds().Dx()/2, s.water.Bounds().Dy()/2
	x, y := (q%2)*w, (q/2)*h
	return image.Rect(x, y, x+w, y+h)
}

// waterCell is the part of s's water layer q cell (x, y) is painted in.
func (s *groundSheet) waterCell(q, x, y int) image.Rectangle {
	o := s.waterLayer(q).Min
	x0, y0 := o.X+x*s.wpx, o.Y+y*s.wpx
	return image.Rect(x0, y0, x0+s.wpx, y0+s.wpx)
}

// resetWater starts s's water canvases over.
func (l *Painter) resetWater(s *groundSheet) {
	for q := range s.wcanvas {
		s.wcanvas[q].Reset(nil)
	}
}

// waterCorners is where the cell-local world corners w of cell i lie on s's water layer q.
func (l *Painter) waterCorners(s *groundSheet, q, i int, w render.World) render.Corners {
	cols := int(l.sq.Cols)
	r := s.waterCell(q, i%cols, i/cols)
	kx, ky := float32(s.wpx)/float32(l.cellW), float32(s.wpx)/float32(l.cellH)
	var c render.Corners
	for k, p := range w {
		c[k] = [2]float32{float32(r.Min.X) + p[0]*kx, float32(r.Min.Y) + p[1]*ky}
	}
	return c
}

// flowOf is a flow as the water's layers hold it: over twice water.FlowSpan, a half for none.
func flowOf(v float32) float32 {
	return min(max(v/(2*water.FlowSpan)+0.5, 0), 1)
}

// layWaterBase adds to s's water canvases cell i's own water, where its base shines: running
// down its slope where the cells round it run (tile.Flow), still water else, over its whole cell.
func (l *Painter) layWaterBase(s *groundSheet, i int) {
	cols := int(l.sq.Cols)
	c, ok := l.cellAt(int64(i%cols), int64(i/cols))
	if !ok {
		return
	}
	base := l.base(c)
	if base.shine <= 0 {
		return
	}
	box := render.Box(0, 0, float32(l.cellW), float32(l.cellH))
	if flow, runs := l.cellTile(c).Flow(); runs {
		var v render.Shade
		for k, f := range flow {
			v[k] = render.Light{flowOf(f[0]), flowOf(f[1]), 1}
		}
		s.wcanvas[water.FlowLayer].Sprite(render.Ground, 0, white, 0, l.waterCorners(s, water.FlowLayer, i, box), v)
		s.wcanvas[water.ShineLayer].Sprite(render.Ground, 0, white, 0, l.waterCorners(s, water.ShineLayer, i, box), render.Lit(render.Light{base.shine, 0, 0}))
		return
	}
	s.wcanvas[water.ShineLayer].Sprite(render.Ground, 0, white, 0, l.waterCorners(s, water.ShineLayer, i, box), render.Lit(render.Light{0, base.shine, 1}))
}

// layWater adds to s's water canvases what of cell i covers the water or runs over it: its blends
// covering it, the pieces of its ways and crossings running with water or covering it, and the
// glint where a way turns into water — each where it lies, faded as it fades, as the tiles lay it.
func (l *Painter) layWater(s *groundSheet, i int, blends []BlendPiece, ways []WayPiece) {
	dark := render.Even(0)
	lay := func(q int, w render.World, shade render.Shade, faded bool, weight [4]float32, soft float32) {
		c := l.waterCorners(s, q, i, w)
		if faded {
			s.wcanvas[q].SpriteBlend(render.Ground, 0, white, 0, c, shade, weight, soft)
			return
		}
		s.wcanvas[q].Sprite(render.Ground, 0, white, 0, c, shade)
	}
	for _, p := range blends {
		lay(water.FlowLayer, p.World, dark, true, p.Weight, p.Soft)
		lay(water.ShineLayer, p.World, dark, true, p.Weight, p.Soft)
	}
	for _, p := range ways {
		soft := p.Soft
		if soft <= 0 {
			soft = 0.5
		}
		var flow, shine, mouth render.Shade
		runs, glints := false, false
		for k := range 4 {
			m := float32(0)
			if p.Mixes {
				m = p.Mix[k]
			}
			run, glint := p.Shine*(1-m), p.MixShine*m
			runs, glints = runs || run > 0, glints || glint > 0
			flow[k] = render.Light{flowOf(p.Flow[k][0]), flowOf(p.Flow[k][1]), 1}
			shine[k] = render.Light{run, 0, 0}
			mouth[k] = render.Light{glint, 1, 0}
		}
		if p.Shine <= 0 || !runs {
			flow, shine = dark, dark // what runs over the water without water covers it
		}
		if p.Faded || !glints {
			mouth = dark
		}
		lay(water.FlowLayer, p.World, flow, p.Faded, p.Weight, soft)
		lay(water.ShineLayer, p.World, shine, p.Faded, p.Weight, soft)
		lay(water.MouthLayer, p.World, mouth, p.Faded, p.Weight, soft)
	}
}
