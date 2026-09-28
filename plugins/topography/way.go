package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/render"
	"math"
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
	Flow   Flow
	Across bool
	Corner [2]float32
	// Weight is how much of the way shows at each corner, 1 all of it: a way fading out, or lying
	// under water, is drawn blended (render.Frame.SpriteBlend) down to nothing, over Soft (a half
	// where 0).
	Weight [4]float32
	Faded  bool
	Soft   float32
	// Mix is how far the way's look has turned at each corner into MixSprite, its Style's MixWith,
	// where Mixes: glazed over it (render.Frame.Glaze), its water running the less and glinting as
	// that kind's does, as shiny as MixShine, the more.
	Mix       [4]float32
	MixSprite render.SpriteID
	Mixes     bool
	MixShine  float32
}

// Way is the tile's board.Way cut into the pieces it is drawn in, its water shining as far as its
// wayDetail. Each way out ends halfway to the
// neighbour it runs on to, as wide there as the mean of the two ways; the two out to the widest
// neighbours are one band curving from the one end to the other round the cell's middle, and any
// other joins it curving in to its middle, so a winding stream bends smoothly from cell to cell:
// where two cells meet, both curves run straight along the line between their middles. A way out
// to one neighbour alone runs straight to the middle and ends square across itself. A band
// running on slantwise leaves the cell for the two either side the last stretch before the corner,
// as long as it is half wide: those pieces are Across it. A way fading out shows the less the more
// it has faded, the ends of a band as much as the mean of the two ways there, and where it runs
// level its water runs on the way it fades: a river running out into the sea. Good until the next
// call; nothing where no way runs.
func (t *tile) Way() []WayPiece { return t.lanePieces(false) }

// Crossing is the tile's board.Crossing cut into pieces as its Way is. Good until the next call;
// nothing where nothing crosses.
func (t *tile) Crossing() []WayPiece { return t.lanePieces(true) }

// lanePieces is Way's, or Crossing's where cross.
func (t *tile) lanePieces(cross bool) []WayPiece {
	r := t.r
	r.ways = r.ways[:0]
	if top := r.topOf(t.ID); !cross && !top.way.Runs() || cross && !top.cross.Runs() {
		return nil
	}
	b := r.bakeOf(t)
	light, lit, w, h := t.Light(), t.sunlit(), t.X1-t.X0, t.Y1-t.Y0
	detail := t.wayDetail()
	pieces := b.ways
	switch {
	case cross && t.px() < nearCell:
		pieces = b.farCrossings
	case cross:
		pieces = b.crossings
	case t.px() < nearCell:
		pieces = b.farWays // a curve in fewer, longer pieces: a cell spans a few pixels
	}
	sea := t.Detail() // what a way turns into glints as far as the ground's water
	for _, p := range pieces {
		p.Shine *= detail
		p.MixShine *= sea
		for k, at := range p.World {
			u, v := at[0]/w, at[1]/h
			p.Light[k], p.Lit[k] = mixLight(light, u, v), mix4(lit, u, v)
		}
		p.World = shift(p.World, t.X0, t.Y0)
		p.Corner[0], p.Corner[1] = p.Corner[0]+t.X0, p.Corner[1]+t.Y0
		r.ways = append(r.ways, p)
	}
	return r.ways
}

// wayDetail is how much a way's water shines, 0 to 1: on a square grid none where the tile is
// dressed from the ground sheet, fading in above it; elsewhere as far as the tile's Detail.
func (t *tile) wayDetail() float32 {
	if !t.r.square {
		return t.Detail()
	}
	return min(max((t.px()-bakeCell)/(bakeCell/2), 0), 1)
}

// wayAnew works out the tile's board.Way into out, or its board.Crossing where cross, in no light;
// its curves in fewer pieces unless fine.
func (t *tile) wayAnew(out []WayPiece, fine, cross bool) []WayPiece {
	r := t.r
	top := r.topOf(t.ID)
	w := top.way
	if cross {
		w = top.cross
	}
	if !w.Runs() {
		return out
	}
	cx, cy := (t.X0+t.X1)/2, (t.Y0+t.Y1)/2
	shine := float32(0)
	if r.quasi3D {
		shine = w.shine
	}
	ground := func(x, y float32) float32 {
		return float32(r.relief.GroundAt(geom.NewVec(float64(x), float64(y))))
	}
	piece := func(world render.World) WayPiece {
		p := WayPiece{World: world, Sprite: w.Kind.SpriteID, Shine: shine, MixSprite: w.mix, Mixes: w.mixes}
		if r.quasi3D {
			p.MixShine = w.mixShine
		}
		for k, at := range world {
			p.Z[k] = ground(at[0], at[1])
		}
		return p
	}
	steps := func(n int) int {
		if fine {
			return n
		}
		return max(n/3, 1)
	}
	h := w.Width / 2
	var outs [8]wayOut
	n := 0
	for i := range 8 {
		if w.Links&(1<<i) == 0 {
			continue
		}
		nb, ok := board.Toward(r.board, t.ID, i)
		if !ok {
			continue
		}
		ex, ey := t.toward(nb, i)
		o := wayOut{x: cx + ex, y: cy + ey, half: h, fade: w.Fade / 2, mix: w.Mix, slant: r.square && i >= 4}
		if other, ok := r.partner(t.ID, nb); ok {
			o.half, o.wide, o.fade, o.mix = (w.Width+other.Width)/4, other.Width, (w.Fade+other.Fade)/2, (w.Mix+other.Mix)/2
		} else if r.topOf(nb).under {
			// out into water: the way the water leaves, so wider than any way in and always the
			// stem, running on to the water's middle under it
			o.fade, o.wide = w.Fade, 2*w.Width
			o.into = &wayOut{x: cx + 2*ex, y: cy + 2*ey, half: h, fade: w.Fade, mix: 1}
		}
		outs[n], n = o, n+1
	}
	// band lays the curve from a round (mx, my) to b, half wide at its ends as they say and h at
	// its middle, in steps pieces, the water running down it from the higher end
	band := func(a, b wayOut, mx, my float32, steps int) {
		at := func(u float32) (x, y, tx, ty, half, shows, mix float32) {
			v := 1 - u
			x = v*v*a.x + 2*u*v*mx + u*u*b.x
			y = v*v*a.y + 2*u*v*my + u*u*b.y
			tx, ty = 2*v*(mx-a.x)+2*u*(b.x-mx), 2*v*(my-a.y)+2*u*(b.y-my)
			if l := float32(math.Hypot(float64(tx), float64(ty))); l > 0 {
				tx, ty = tx/l, ty/l
			}
			return x, y, tx, ty, v*v*a.half + 2*u*v*h + u*u*b.half, 1 - (v*v*a.fade + 2*u*v*w.Fade + u*u*b.fade),
				v*v*a.mix + 2*u*v*w.Mix + u*u*b.mix
		}
		length := float32(math.Hypot(float64(a.x-mx), float64(a.y-my)) + math.Hypot(float64(b.x-mx), float64(b.y-my)))
		speed := float32(0)
		if length > 0 && w.flow > 0 {
			fall := (ground(a.x, a.y) - ground(b.x, b.y)) / length
			// level water runs on the way the way fades: out into the sea
			if math.Abs(float64(fall)) < stillFall && a.fade != b.fade {
				fall = stillFall
				if b.fade < a.fade {
					fall = -stillFall
				}
			}
			if fall != 0 {
				speed = w.flow * float32(math.Sqrt(math.Abs(float64(fall))))
				if fall < 0 {
					speed = -speed
				}
			}
		}
		x0, y0, tx0, ty0, h0, s0, m0 := at(0)
		for k := 1; k <= steps; k++ {
			x1, y1, tx1, ty1, h1, s1, m1 := at(float32(k) / float32(steps))
			p := piece(render.World{{x0 - ty0*h0, y0 + tx0*h0}, {x1 - ty1*h1, y1 + tx1*h1}, {x0 + ty0*h0, y0 - tx0*h0}, {x1 + ty1*h1, y1 - tx1*h1}})
			p.Flow = Flow{{tx0 * speed, ty0 * speed}, {tx1 * speed, ty1 * speed}, {tx0 * speed, ty0 * speed}, {tx1 * speed, ty1 * speed}}
			p.Weight, p.Faded = [4]float32{s0, s1, s0, s1}, s0 < 1 || s1 < 1
			p.Mix = [4]float32{m0, m1, m0, m1}
			// the stretch by a corner a band runs slantwise through reaches into the cells either side
			near := func(e wayOut) bool {
				return e.slant && min(math.Hypot(float64(x0-e.x), float64(y0-e.y)), math.Hypot(float64(x1-e.x), float64(y1-e.y))) < float64(e.half)
			}
			switch {
			case near(a):
				p.Across, p.Corner = true, [2]float32{a.x, a.y}
			case near(b):
				p.Across, p.Corner = true, [2]float32{b.x, b.y}
			}
			out = append(out, p)
			x0, y0, tx0, ty0, h0, s0, m0 = x1, y1, tx1, ty1, h1, s1, m1
		}
	}
	switch n {
	case 0:
		p := piece(render.World{{cx - h, cy - h}, {cx + h, cy - h}, {cx - h, cy + h}, {cx + h, cy + h}})
		p.Weight, p.Faded = [4]float32{1 - w.Fade, 1 - w.Fade, 1 - w.Fade, 1 - w.Fade}, w.Fade > 0
		p.Mix = [4]float32{w.Mix, w.Mix, w.Mix, w.Mix}
		out = append(out, p)
	case 1:
		// straight out to the one neighbour, from square across itself behind the middle
		o := outs[0]
		ax, ay := o.x-cx, o.y-cy
		l := float32(math.Hypot(float64(ax), float64(ay)))
		back := wayOut{x: cx - ax/l*h, y: cy - ay/l*h, half: h, fade: w.Fade, mix: w.Mix}
		if w.Fade > 0 {
			back.fade = 1 // a way fading out ends in nothing
		}
		mid := wayOut{x: cx, y: cy, half: h, fade: w.Fade, mix: w.Mix}
		// round a point on the line, the curve runs straight
		band(back, mid, (back.x+mid.x)/2, (back.y+mid.y)/2, 1)
		band(mid, o, (mid.x+o.x)/2, (mid.y+o.y)/2, steps(4))
	default:
		// the stem: the ways out to the two widest neighbours; any other joins it halfway
		a, b := 0, 1
		if outs[b].wide > outs[a].wide {
			a, b = b, a
		}
		for i := 2; i < n; i++ {
			switch {
			case outs[i].wide > outs[a].wide:
				a, b = i, a
			case outs[i].wide > outs[b].wide:
				b = i
			}
		}
		band(outs[a], outs[b], cx, cy, steps(6))
		// the middle of the stem's curve, where the others join it
		jx := 0.25*outs[a].x + 0.5*cx + 0.25*outs[b].x
		jy := 0.25*outs[a].y + 0.5*cy + 0.25*outs[b].y
		for i := range n {
			if i == a || i == b {
				continue
			}
			band(outs[i], wayOut{x: jx, y: jy, half: min(outs[i].half, h), fade: w.Fade, mix: w.Mix}, cx, cy, steps(4))
		}
	}
	// a way running out into water runs on under it, over the water's cell: at its depth
	for _, o := range outs[:n] {
		if o.into == nil {
			continue
		}
		from := len(out)
		band(o, *o.into, (o.x+o.into.x)/2, (o.y+o.into.y)/2, steps(2))
		for k := from; k < len(out); k++ {
			out[k].Across, out[k].Corner = true, [2]float32{o.x, o.y}
		}
	}
	// the water lies over a way, as over the grounds round it: it shows only where the land does
	soft := top.spread
	if soft <= 0 {
		soft = 0.25
	}
	for k := range out {
		p := &out[k]
		var land [4]float32
		for c, at := range p.World {
			land[c] = r.landAt(at[0], at[1])
		}
		if min(land[0], land[1], land[2], land[3]) >= 1 {
			continue
		}
		if !p.Faded {
			p.Weight = [4]float32{1, 1, 1, 1}
		}
		for c := range land {
			p.Weight[c] = min(p.Weight[c], land[c])
		}
		p.Faded, p.Soft = true, soft
	}
	return out
}

// partner is what runs across nb that c's way or crossing running on to it meets: whichever of its
// Way and Crossing runs back to c, its Way where neither does; false where nothing runs there.
func (l *dresser) partner(c, nb board.CellID) (board.Way, bool) {
	top := l.topOf(nb)
	if back, ok := board.Link(l.board, nb, c); ok && top.way.Links&back == 0 && top.cross.Links&back != 0 {
		return top.cross.Way, true
	}
	return top.way.Way, top.way.Runs()
}

// landAt is how much of the ground at the world point x, y is land, as the grounds round a coast
// are laid over the water: the share of the cells meeting there that do not lie under, blended
// across the tile as its blends are; 1 inland, 1 off a square grid.
func (l *dresser) landAt(x, y float32) float32 {
	if !l.square {
		return 1
	}
	size := float32(l.sq.Cell)
	fx, fy := float64(x/size), float64(y/size)
	cx, cy := math.Floor(fx), math.Floor(fy)
	c, ok := l.cellAt(int64(cx), int64(cy))
	if !ok {
		return 1
	}
	near := l.around(c)
	w := weigh(&near, func(n *cellTop) bool { return !n.under })
	gu, gv := 2*(float32(fx-cx)), 2*(float32(fy-cy))
	i, j := min(int(gu), 1), min(int(gv), 1)
	u, v := gu-float32(i), gv-float32(j)
	return mix4([4]float32{w[j][i], w[j][i+1], w[j+1][i], w[j+1][i+1]}, u, v)
}

// wayOut is one way out of a way's cell: where it ends, halfway to its neighbour, how half wide it
// is there, how faded and how far its look has turned, how wide the neighbour's way is, whether it
// runs slantwise through a corner, and where it runs on to under water.
type wayOut struct {
	x, y, half, fade, mix, wide float32
	slant                       bool
	into                        *wayOut // where it runs on to under water
}

// stillFall is the fall, rise over run, under which water lies level: there a way's water runs on
// the way it fades, as if it fell so much.
const stillFall = 0.01

// toward is the way from the middle of the tile to halfway to its neighbour n, the grid's i-th
// direction.
func (t *tile) toward(n board.CellID, i int) (float32, float32) {
	if t.r.square {
		return float32(squareDirs[i][0]) * (t.X1 - t.X0) / 2, float32(squareDirs[i][1]) * (t.Y1 - t.Y0) / 2
	}
	a, b := t.r.board.CellCenter(t.ID), t.r.board.CellCenter(n)
	return float32(b.X-a.X) / 2, float32(b.Y-a.Y) / 2
}

// wayTier puts what runs across the cells over every tile: from above a way reaching into a
// neighbour's cell is never covered by its tile.
const wayTier = render.Ground + 5

// crossingTier puts what crosses over a way over it: a bridge over its river.
const crossingTier = wayTier + 1

// DrawWay draws what runs across the tile over it, at depth: each piece in its light, the clouds'
// shadows over it and, where it shines, its water running. A piece reaching across a corner takes
// the depth of the nearest of the four cells meeting there, so none of them covers it.
func (t *tile) DrawWay(f *render.Frame, cam camera.Camera, depth float32) {
	t.drawLane(f, cam, depth, wayTier, t.Way())
	t.drawLane(f, cam, depth, crossingTier, t.Crossing())
}

// drawLane draws pieces of what runs across the tile on tier, at depth.
func (t *tile) drawLane(f *render.Frame, cam camera.Camera, depth float32, tier render.Tier, pieces []WayPiece) {
	w, h := (t.X1-t.X0)/2, (t.Y1-t.Y0)/2
	for _, p := range pieces {
		var corners render.Corners
		for k, at := range p.World {
			corners[k][0], corners[k][1] = cam.Project(at[0], at[1], p.Z[k])
		}
		d := depth
		if p.Across {
			for _, o := range [4][2]float32{{-w, -h}, {w, -h}, {-w, h}, {w, h}} {
				x, y := p.Corner[0]+o[0], p.Corner[1]+o[1]
				d = max(d, cam.Depth(x, y, float32(t.r.relief.GroundAt(geom.NewVec(float64(x), float64(y))))))
			}
		}
		mark := p.draw(f, tier, d, t.Atlas, corners, p.Light)
		lit := p.Lit
		if t.r.cover > 0 {
			cloud := t.r.cloudsAt(p.World)
			f.OvercastOn(mark, p.World, cloud)
			lit = t.r.shaded(lit, cloud)
		}
		// the water runs the less, and glints as what it turns into the more, the further it has turned
		var runs, glints [4]float32
		for k, m := range p.Mix {
			if !p.Mixes {
				m = 0
			}
			runs[k], glints[k] = p.Shine*(1-m), p.MixShine*m
		}
		if p.Shine > 0 && max(runs[0], runs[1], runs[2], runs[3]) > 0 {
			Stream(f, p.World, runs, lit, p.Flow)
		}
		if !p.Faded && max(glints[0], glints[1], glints[2], glints[3]) > 0 {
			Glint(f, p.World, glints, lit, Shore{})
		}
	}
}

// draw draws the piece over corners in shade, blended where it fades or lies under water, what it
// turns into glazed over it, and gives its sprite.
func (p *WayPiece) draw(f *render.Frame, tier render.Tier, depth float32, atlas render.AtlasSource, corners render.Corners, shade render.Shade) render.Mark {
	mixes := p.Mixes && max(p.Mix[0], p.Mix[1], p.Mix[2], p.Mix[3]) > 0
	if p.Faded {
		soft := p.Soft
		if soft <= 0 {
			soft = 0.5
		}
		f.SpriteBlend(tier, depth, atlas, p.Sprite, corners, shade, p.Weight, soft)
		drawn := f.Last()
		if mixes {
			f.GlazeBlend(tier, depth, atlas, p.MixSprite, corners, shade, p.Weight, soft, p.Mix)
		}
		return drawn
	}
	f.Sprite(tier, depth, atlas, p.Sprite, corners, shade)
	drawn := f.Last()
	if mixes {
		f.Glaze(tier, depth, atlas, p.MixSprite, corners, shade, p.Mix)
	}
	return drawn
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

// cellBake is what a cell's tile draws over itself, worked out once while the cells round it stay
// as they are: its blends and its way, near and far, placed as if the tile's top-left corner were
// at 0, 0, in no light yet.
type cellBake struct {
	ver          uint64 // one more than the newest version round the cell when baked; 0 never
	seen         uint64 // the board's count of changes when last found as it was
	blends       []BlendPiece
	ways         []WayPiece
	farWays      []WayPiece
	crossings    []WayPiece
	farCrossings []WayPiece
}

// bakeOf is t's bake, worked out anew when a cell round it has changed since.
func (l *dresser) bakeOf(t *tile) *cellBake {
	i, _ := l.ordinal(t.ID)
	b := &l.bakes[i]
	if b.ver != 0 && b.seen == l.board.Changes() { // nothing on the board has changed since
		return b
	}
	b.seen = l.board.Changes()
	v := l.board.CellVersion(t.ID)
	for d := range 8 {
		if n, ok := board.Toward(l.board, t.ID, d); ok {
			v = max(v, l.board.CellVersion(n))
		}
	}
	if b.ver == v+1 {
		return b
	}
	b.ver = v + 1
	b.blends = t.blendsAnew(b.blends[:0])
	b.ways, b.farWays = t.wayAnew(b.ways[:0], true, false), t.wayAnew(b.farWays[:0], false, false)
	b.crossings, b.farCrossings = t.wayAnew(b.crossings[:0], true, true), t.wayAnew(b.farCrossings[:0], false, true)
	for k := range b.blends {
		b.blends[k].World = shift(b.blends[k].World, -t.X0, -t.Y0)
	}
	for _, ways := range [4][]WayPiece{b.ways, b.farWays, b.crossings, b.farCrossings} {
		for k := range ways {
			p := &ways[k]
			p.World = shift(p.World, -t.X0, -t.Y0)
			p.Corner[0], p.Corner[1] = p.Corner[0]-t.X0, p.Corner[1]-t.Y0
		}
	}
	return b
}

// shift is w moved by dx, dy.
func shift(w render.World, dx, dy float32) render.World {
	for k := range w {
		w[k][0], w[k][1] = w[k][0]+dx, w[k][1]+dy
	}
	return w
}
