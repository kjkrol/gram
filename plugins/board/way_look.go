package board

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// WayPiece is one piece of what runs across a tile: a band from the cell's middle out towards a
// neighbour, or the square that joins the bands where the way turns. World is where its corners lie
// — top-left, top-right, bottom-left, bottom-right — Z the ground under them; it is drawn with its
// way's sprite in Light and, where the way shines, as water running at Flow in Lit of the sun. A
// piece Across the corner at Corner, the end of a band running on slantwise, reaches into the two
// cells either side of it too.
type WayPiece struct {
	World  render.World
	Z      [4]float32
	Sprite render.SpriteID
	Light  render.Shade
	Shine  float32
	Lit    [4]float32
	Flow   render.Flow
	Across bool
	Corner [2]float32
}

// Way is the tile's Way cut into the pieces it is drawn in: from the middle of the cell a band out
// to halfway to each neighbour it runs on to, its own Width at the middle and at the far end the
// mean of its and the neighbour's, so a widening river has no steps; and where it turns, a square
// joining the bands. Good until the next call; nothing where no way runs.
func (t *Tile) Way() []WayPiece {
	r := t.r
	top := r.topOf(t.ID)
	w := top.way
	r.ways = r.ways[:0]
	if !w.Runs() {
		return nil
	}
	cx, cy := (t.X0+t.X1)/2, (t.Y0+t.Y1)/2
	light, lit := t.Light(), t.sunlit()
	shine := float32(0)
	if r.board.quasi3D {
		shine = float32(w.Kind.Shine)
	}
	piece := func(world render.World) WayPiece {
		p := WayPiece{World: world, Sprite: w.Kind.SpriteID, Shine: shine}
		for k, at := range world {
			u, v := (at[0]-t.X0)/(t.X1-t.X0), (at[1]-t.Y0)/(t.Y1-t.Y0)
			p.Z[k] = float32(r.board.GroundAt(geom.NewVec(float64(at[0]), float64(at[1]))))
			p.Light[k] = mixLight(light, u, v)
			p.Lit[k] = mix4(lit, u, v)
		}
		return p
	}
	bands := 0
	var out, run [2]float32 // the ways out, summed to tell a line from a bend; the water's runs
	for i := range 8 {
		if w.Links&(1<<i) == 0 {
			continue
		}
		n, ok := Toward(r.board, t.ID, i)
		if !ok {
			continue
		}
		ex, ey := t.toward(n, i)
		ex, ey = cx+ex, cy+ey
		ax, ay := ex-cx, ey-cy
		length := float32(math.Hypot(float64(ax), float64(ay)))
		if length == 0 {
			continue
		}
		ax, ay = ax/length, ay/length
		px, py := -ay, ax
		near, far := w.Width/2, w.Width/2
		if o := r.topOf(n).way; o.Runs() {
			far = (w.Width + o.Width) / 4
		}
		// the water runs down the band, as fast as the Flow by the square root of its fall
		var flow render.Flow
		fall := (float32(r.board.GroundAt(geom.NewVec(float64(cx), float64(cy)))) - float32(r.board.GroundAt(geom.NewVec(float64(ex), float64(ey))))) / length
		if w.Kind.Flow > 0 && fall != 0 {
			speed := float32(w.Kind.Flow) * float32(math.Sqrt(math.Abs(float64(fall))))
			if fall < 0 {
				speed = -speed
			}
			for k := range flow {
				flow[k] = [2]float32{ax * speed, ay * speed}
			}
			run[0], run[1] = run[0]+ax*speed, run[1]+ay*speed
		}
		band := func(sx, sy, from, tx, ty, to float32) WayPiece {
			p := piece(render.World{{sx + px*from, sy + py*from}, {tx + px*to, ty + py*to}, {sx - px*from, sy - py*from}, {tx - px*to, ty - py*to}})
			p.Flow = flow
			return p
		}
		// a band running on slantwise leaves the cell for the two either side the last stretch
		// before the corner, as long as it is half wide: that stretch is a piece of its own
		if cut := min(max(near, far), length); r.board.square != nil && i >= 4 && cut > 0 {
			mx, my := ex-ax*cut, ey-ay*cut
			mid := near + (far-near)*(1-cut/length)
			r.ways = append(r.ways, band(cx, cy, near, mx, my, mid))
			p := band(mx, my, mid, ex, ey, far)
			p.Across, p.Corner = true, [2]float32{ex, ey}
			r.ways = append(r.ways, p)
		} else {
			r.ways = append(r.ways, band(cx, cy, near, ex, ey, far))
		}
		bands++
		out[0], out[1] = out[0]+ax, out[1]+ay
	}
	// a band straight through needs nothing more; anything else is joined by a square
	if bands != 2 || math.Hypot(float64(out[0]), float64(out[1])) > 1e-3 {
		h := w.Width / 2
		p := piece(render.World{{cx - h, cy - h}, {cx + h, cy - h}, {cx - h, cy + h}, {cx + h, cy + h}})
		if bands > 0 {
			for k := range p.Flow {
				p.Flow[k] = [2]float32{run[0] / float32(bands), run[1] / float32(bands)}
			}
		}
		r.ways = append(r.ways, p)
	}
	return r.ways
}

// toward is the way from the middle of the tile to halfway to its neighbour n, the grid's i-th
// direction.
func (t *Tile) toward(n CellID, i int) (float32, float32) {
	if t.r.board.square != nil {
		return float32(squareDirs[i][0]) * (t.X1 - t.X0) / 2, float32(squareDirs[i][1]) * (t.Y1 - t.Y0) / 2
	}
	a, b := t.r.board.CellCenter(t.ID), t.r.board.CellCenter(n)
	return float32(b.X-a.X) / 2, float32(b.Y-a.Y) / 2
}

// wayTier puts what runs across the cells over every tile: from above a way reaching into a
// neighbour's cell is never covered by its tile.
const wayTier = render.Ground + 5

// DrawWay draws what runs across the tile over it, at depth: each piece in its light, the clouds'
// shadows over it and, where it shines, its water running. A piece reaching across a corner takes
// the depth of the nearest of the four cells meeting there, so none of them covers it.
func (t *Tile) DrawWay(f *render.Frame, cam camera.Camera, depth float32) {
	w, h := (t.X1-t.X0)/2, (t.Y1-t.Y0)/2
	for _, p := range t.Way() {
		var corners render.Corners
		for k, at := range p.World {
			corners[k][0], corners[k][1] = cam.Project(at[0], at[1], p.Z[k])
		}
		d := depth
		if p.Across {
			for _, o := range [4][2]float32{{-w, -h}, {w, -h}, {-w, h}, {w, h}} {
				x, y := p.Corner[0]+o[0], p.Corner[1]+o[1]
				d = max(d, cam.Depth(x, y, float32(t.r.board.GroundAt(geom.NewVec(float64(x), float64(y))))))
			}
		}
		f.Sprite(wayTier, d, t.Atlas, p.Sprite, corners, p.Light)
		f.OvercastAt(p.World)
		if p.Shine > 0 {
			f.Stream(p.World, p.Shine, p.Lit, p.Flow)
		}
	}
}

// mixLight is the light at (u, v) across a tile lit s at its corners.
func mixLight(s render.Shade, u, v float32) render.Light {
	var out render.Light
	for c := range out {
		out[c] = mix4([4]float32{s[0][c], s[1][c], s[2][c], s[3][c]}, u, v)
	}
	return out
}

// mix4 is the value at (u, v) across a tile with corners at c.
func mix4(c [4]float32, u, v float32) float32 {
	top := c[0] + (c[1]-c[0])*u
	bottom := c[2] + (c[3]-c[2])*u
	return top + (bottom-top)*v
}
