package painter

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// BlendPiece is a quarter of a tile over which a neighbour's ground runs in, its Sprite drawn
// where Weight, blended between the corners, is over a half, fading in over Soft either side.
// World's corners are top-left, top-right, bottom-left, bottom-right; Z is the ground under them.
type BlendPiece struct {
	World  render.World
	Z      [4]float32
	Sprite render.SpriteID
	Light  render.Shade
	Weight [4]float32
	Soft   float32
}

// Blends are the grounds round the tile laid over it along the lines their cells draw. Good
// until the next call; nil off a square grid.
func (t *tile) Blends() []BlendPiece {
	r := t.r
	r.blends = r.blends[:0]
	if !r.square {
		return nil
	}
	b := r.bakeOf(t)
	if len(b.blends) == 0 {
		return nil
	}
	light, w, h := t.Light(), t.X1-t.X0, t.Y1-t.Y0
	for _, p := range b.blends {
		for k, at := range p.World {
			p.Light[k] = mixLight(light, at[0]/w, at[1]/h)
		}
		p.World = shift(p.World, t.X0, t.Y0)
		r.blends = append(r.blends, p)
	}
	return r.blends
}

func (t *tile) blendsAnew(out []BlendPiece) []BlendPiece {
	r := t.r
	if !r.square {
		return out
	}
	mine := r.topOf(t.ID)
	if mine.raised || !mine.under && mine.spread <= 0 {
		return out
	}
	near := r.around(t.ID)
	land := func(n *cellTop) bool { return !n.under && n.spread > 0 && !n.raised }
	top := mine
	if mine.under {
		top = nil
		var kinds [9]*cellTop
		var counts [9]int
		seen, most := 0, 0
		for _, row := range near {
			for _, n := range row {
				if !land(n) {
					continue
				}
				k := 0
				for k < seen && kinds[k].sprite != n.sprite {
					k++
				}
				if k == seen {
					kinds[k], seen = n, seen+1
				}
				if counts[k]++; counts[k] > most {
					top, most = kinds[k], counts[k]
				}
			}
		}
		if top == nil {
			return out
		}
	}
	w, h := t.X1-t.X0, t.Y1-t.Y0
	lay := func(sprite render.SpriteID, soft float32, of func(n *cellTop) bool) {
		weight := weigh(&near, of)
		for qj := range 2 {
			for qi := range 2 {
				q := [4]float32{weight[qj][qi], weight[qj][qi+1], weight[qj+1][qi], weight[qj+1][qi+1]}
				if max(q[0], q[1], q[2], q[3]) <= 0.5-soft {
					continue
				}
				x0, y0 := t.X0+float32(qi)*w/2, t.Y0+float32(qj)*h/2
				p := BlendPiece{World: render.World{{x0, y0}, {x0 + w/2, y0}, {x0, y0 + h/2}, {x0 + w/2, y0 + h/2}},
					Sprite: sprite, Weight: q, Soft: soft}
				for k, at := range p.World {
					p.Z[k] = float32(r.relief.GroundAt(geom.NewVec(float64(at[0]), float64(at[1]))))
				}
				out = append(out, p)
			}
		}
	}
	if base := t.baseTop(); base != top {
		lay(top.sprite, top.spread, func(n *cellTop) bool { return !n.under })
	}
	var done [9]render.SpriteID
	seen := 0
	for _, row := range near {
		for _, other := range row {
			if !land(other) || other.sprite == top.sprite || contains(done[:seen], other.sprite) {
				continue
			}
			done[seen], seen = other.sprite, seen+1
			lay(other.sprite, (top.spread+other.spread)/2, func(n *cellTop) bool { return n.sprite == other.sprite })
		}
	}
	return out
}

func (l *Painter) base(c cell.ID) *cellTop {
	mine := l.topOf(c)
	if !l.square || mine.under || mine.raised || mine.spread <= 0 {
		return mine
	}
	for d := range l.board.Directions() {
		if n, ok := l.board.Toward(c, d); ok && l.topOf(n).under {
			return l.topOf(n)
		}
	}
	return mine
}

// c's own stands in for any cell off the board
func (l *Painter) around(c cell.ID) [3][3]*cellTop {
	mine := l.topOf(c)
	var near [3][3]*cellTop
	for j := range 3 {
		for i := range 3 {
			near[j][i] = mine
		}
	}
	for d := range l.board.Directions() {
		if n, ok := l.board.Toward(c, d); ok {
			da, db := l.board.Offset(d)
			near[db+1][da+1] = l.topOf(n)
		}
	}
	return near
}

// index 0 is the tile's left or top edge, 1 its middle, 2 its right or bottom edge
func weigh(near *[3][3]*cellTop, of func(n *cellTop) bool) [3][3]float32 {
	span := [3][2]int{{0, 1}, {1, 1}, {1, 2}}
	var weight [3][3]float32
	for j := range 3 {
		for i := range 3 {
			in, all := 0, 0
			for cy := span[j][0]; cy <= span[j][1]; cy++ {
				for cx := span[i][0]; cx <= span[i][1]; cx++ {
					all++
					if of(near[cy][cx]) {
						in++
					}
				}
			}
			weight[j][i] = float32(in) / float32(all)
		}
	}
	return weight
}

func contains(ids []render.SpriteID, id render.SpriteID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// DrawBlends draws the neighbours' grounds running into the tile at depth.
func (t *tile) DrawBlends(f *render.Frame, cam camera.Camera, depth float32) {
	for _, p := range t.Blends() {
		if !t.r.inFront(cam, p.World, p.Z) {
			continue // beside or behind the eye, it would be thrown across the screen
		}
		var c render.Corners
		for k, at := range p.World {
			c[k][0], c[k][1] = cam.Project(at[0], at[1], p.Z[k])
		}
		f.SpriteBlend(render.Ground, depth, t.Atlas, p.Sprite, c, p.Light, p.Weight, p.Soft)
	}
}
