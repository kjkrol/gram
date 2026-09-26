package board

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// BlendPiece is a quarter of a tile over which a neighbour's ground runs in: World is where its
// corners lie — top-left, top-right, bottom-left, bottom-right — Z the ground under them; the
// neighbour's sprite is drawn in the tile's Light where Weight, blended between the corners, is
// over a half, fading in over Soft either side of it.
type BlendPiece struct {
	World  render.World
	Z      [4]float32
	Sprite render.SpriteID
	Light  render.Shade
	Weight [4]float32
	Soft   float32
}

// Blends are the grounds round the tile laid over it along the lines their cells draw, not the
// cells' edges. A kind is weighed at the tile's corners, the middles of its sides and its middle by
// the share of the cells meeting there that are of it — a quarter at a corner, a half at a side,
// the tile's own at the middle — and drawn over each quarter of the tile where that weight is over
// a half, fading in over its Spread either side. The weights at a corner or a side are the same
// from every cell meeting there, so a line runs on from tile to tile: a staircase of cells becomes
// a slant, a cell alone a rounded diamond.
//
// A tile next to a kind Under it — the sea — is drawn as that kind (Base) and its own laid over it
// weighed by the share of cells not under; a tile of such a kind has the land round it laid over it
// the same way, in the sprite of the kind most of that land is. Over that, each other kind that
// spreads round the tile is laid weighed by its own share, blending over the mean of its Spread and
// the land's. Nothing blends where the tile's kind keeps its cells square (no Spread, not Under),
// where a kind stands a Height over its ground, nor off a square grid. Good until the next call.
func (t *Tile) Blends() []BlendPiece {
	r := t.r
	r.blends = r.blends[:0]
	sq := r.board.square
	if sq == nil {
		return nil
	}
	mine := r.topOf(t.ID)
	if mine.raised || !mine.under && mine.spread <= 0 {
		return nil
	}
	near := r.around(t.ID)
	land := func(n *cellTop) bool { return !n.under && n.spread > 0 && !n.raised }
	// the kind laid over the base: the tile's own, or on a tile under, the land round it most is
	top := mine
	if mine.under {
		top = nil
		counts := map[render.SpriteID]int{}
		for _, row := range near {
			for _, n := range row {
				if land(n) {
					counts[n.sprite]++
					if top == nil || counts[n.sprite] > counts[top.sprite] {
						top = n
					}
				}
			}
		}
		if top == nil {
			return nil // open water
		}
	}
	w, h := t.X1-t.X0, t.Y1-t.Y0
	light := t.Light()
	lay := func(sprite render.SpriteID, soft float32, of func(n *cellTop) bool) {
		weight := weigh(&near, of)
		for qj := range 2 {
			for qi := range 2 {
				q := [4]float32{weight[qj][qi], weight[qj][qi+1], weight[qj+1][qi], weight[qj+1][qi+1]}
				if q == ([4]float32{}) {
					continue
				}
				x0, y0 := t.X0+float32(qi)*w/2, t.Y0+float32(qj)*h/2
				p := BlendPiece{World: render.World{{x0, y0}, {x0 + w/2, y0}, {x0, y0 + h/2}, {x0 + w/2, y0 + h/2}},
					Sprite: sprite, Weight: q, Soft: soft}
				for k, at := range p.World {
					p.Z[k] = float32(r.board.GroundAt(geom.NewVec(float64(at[0]), float64(at[1]))))
					p.Light[k] = mixLight(light, (at[0]-t.X0)/w, (at[1]-t.Y0)/h)
				}
				r.blends = append(r.blends, p)
			}
		}
	}
	if base := r.base(t.ID); base != top {
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
	return r.blends
}

// base is what c's top is drawn in first: a kind Under it round it where c's own kind spreads, else
// its own.
func (l *Renderer) base(c CellID) *cellTop {
	mine := l.topOf(c)
	sq := l.board.square
	if sq == nil || mine.under || mine.raised || mine.spread <= 0 {
		return mine
	}
	x, y := sq.cellXY(c)
	for _, d := range [8][2]int64{{0, -1}, {0, 1}, {-1, 0}, {1, 0}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		if n, ok := l.board.squareCell(int64(x)+d[0], int64(y)+d[1]); ok && l.topOf(n).under {
			return l.topOf(n)
		}
	}
	return mine
}

// around is the cells round c, c's own standing in for any off the board.
func (l *Renderer) around(c CellID) [3][3]*cellTop {
	x, y := l.board.square.cellXY(c)
	mine := l.topOf(c)
	var near [3][3]*cellTop
	for dy := int64(-1); dy <= 1; dy++ {
		for dx := int64(-1); dx <= 1; dx++ {
			near[dy+1][dx+1] = mine
			if n, ok := l.board.squareCell(int64(x)+dx, int64(y)+dy); ok {
				near[dy+1][dx+1] = l.topOf(n)
			}
		}
	}
	return near
}

// weigh is the share of the cells round a tile that are of, at each of its points across and down:
// 0 its left or top edge, 1 its middle, 2 its right or bottom edge — the tile's own at its middle.
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

// DrawBlends draws the neighbours' grounds running into the tile at depth, and the clouds' shadows
// over them only where they show.
func (t *Tile) DrawBlends(f *render.Frame, cam camera.Camera, depth float32) {
	for _, p := range t.Blends() {
		var c render.Corners
		for k, at := range p.World {
			c[k][0], c[k][1] = cam.Project(at[0], at[1], p.Z[k])
		}
		f.SpriteBlend(render.Ground, depth, t.Atlas, p.Sprite, c, p.Light, p.Weight, p.Soft)
		f.OvercastAt(p.World)
	}
}
