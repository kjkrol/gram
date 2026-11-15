package ribbons

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
)

// Flaw is a place a road is laid wrong: where, what is wrong there and by how much, in world
// units.
type Flaw struct {
	At   geom.Vec
	What string
	Of   string // the road it is on: a road, a junction, an end, a roundabout, a bridge, a tunnel
	By   float32
}

// What a Flaw says.
const (
	Buried = "the ground lies over the road"
	Hole   = "the road is not laid"
	Step   = "the road steps"
	Narrow = "the road's width jumps"
	Line   = "a line lies across the road"
	Ends   = "a line ends"
	Hidden = "a line lies under the road"
)

// metres: how far over a road the ground may lie, how high a road may step between two cells
// or along a stretch too steep, and how far its width may jump there
// how far inside another road's roadway a line lies to lie across it, metres
const deepBy = 0.3

const (
	buriedBy = 0.02
	stepBy   = 0.08
	narrowBy = 0.4
	// how far either side of a line the road may not lie on both
	lineBy = 1
	// how far from another line a line may end
	joinBy = 0.6
	// how far under the roadway a line may lie, and how far over it a deck lies
	hiddenBy = 0.02
	deckBy   = 1.5
	// the steepest a stretch of road lies along its axis, rise over run
	steepBy = 0.35
)

// of names the road a node is
func (n *node) of() string {
	switch {
	case n.sunk:
		return "tunnel"
	case n.round != 0:
		return "roundabout"
	case n.deck && n.head:
		return "bridge head"
	case n.deck:
		return "bridge"
	case len(n.arms) >= 3:
		return "junction"
	case len(n.arms) < 2:
		return "end"
	}
	return "road"
}

// laid is a piece of road as it is drawn: its corners, each with its height
type laid struct {
	at    [4]geom.Vec
	z     [4]float32
	round float32 // a plate's radius
	// where it is drawn: of its sector's pieces, the later over the earlier at one height
	sector, index int
	// the road it is of, and the part of it
	owner, part int
	// a stretch of road: its axis and half its width at either end
	strip  bool
	p0, p1 geom.Vec
	w0, w1 float32
}

// Audit lays the roads over the ground and says where they are laid wrong: under the ground,
// with a hole, a step or a jump in width where one cell's road meets the next's.
func (r *Renderer) Audit() []Flaw {
	ground := r.cfg.Ground()
	if ground == nil || r.cfg.Board == nil {
		return nil
	}
	r.audit = true
	r.lay(ground)
	r.audit = false
	m := r.cfg.Metre
	by := map[int][]laid{}
	// every piece of road by the squares it lies in, a line's pieces and the stretches marked
	// along their edges
	grid := map[[2]int][]laid{}
	type mark struct {
		at, way geom.Vec
		z, long float32
		of      int
		// the piece it is drawn with, and whether it is a line laid over the road
		sector, index int
		over          bool
	}
	var marks []mark
	var flaws []Flaw
	for sector, all := range r.pieces {
		for i := 0; i < len(all); i += floats {
			p := all[i : i+floats]
			kind := int(p[13])
			if kind&bank != 0 {
				continue
			}
			if [3]float32(p[16:19]) == lineRGB {
				// a line along an edge, not the dashes a road gives way at
				d := geom.NewVec(p[2]-p[0], p[3]-p[1])
				w := f32.Hypot(p[4], p[5])
				if l := f32.Hypot(d.X, d.Y); l > 0 && w > 0 && w < 1.5*lineHalf*m {
					marks = append(marks, mark{
						at:     geom.NewVec((p[0]+p[2])/2, (p[1]+p[3])/2),
						way:    geom.NewVec(d.X/l, d.Y/l),
						z:      (p[8] + p[9]) / 2,
						of:     r.owners[sector][i/floats],
						sector: sector, index: i / floats, over: true,
					})
				}
				continue
			}
			l := laid{
				at: [4]geom.Vec{
					geom.NewVec(p[0]-p[4], p[1]-p[5]),
					geom.NewVec(p[0]+p[4], p[1]+p[5]),
					geom.NewVec(p[2]+p[6], p[3]+p[7]),
					geom.NewVec(p[2]-p[6], p[3]-p[7]),
				},
				z: [4]float32{p[8], p[8], p[9], p[9]},
			}
			if kind&disc != 0 {
				l.round = gap(geom.NewVec(p[0], p[1]), geom.NewVec(p[2], p[3])) / 2
			}
			// a stretch of road steeper along its axis than any road climbs is a step
			run := gap(geom.NewVec(p[0], p[1]), geom.NewVec(p[2], p[3]))
			strip := (p[4] != 0 || p[5] != 0) && (p[6] != 0 || p[7] != 0) && kind&disc == 0
			if rise := f32.Abs(p[9] - p[8]); strip && rise > stepBy*m && rise > steepBy*run {
				flaws = append(flaws, Flaw{At: geom.NewVec((p[0]+p[2])/2, (p[1]+p[3])/2),
					What: Step, Of: r.nodes[r.owners[sector][i/floats]].of(), By: rise})
			}
			l.sector, l.index = sector, i/floats
			l.strip, l.p0, l.p1 = strip, geom.NewVec(p[0], p[1]), geom.NewVec(p[2], p[3])
			l.w0, l.w1 = f32.Hypot(p[4], p[5]), f32.Hypot(p[6], p[7])
			owner := r.owners[sector][i/floats]
			by[owner] = append(by[owner], l)
			lo, hi := l.at[0], l.at[0]
			for _, c := range l.at {
				lo = geom.NewVec(min(lo.X, c.X), min(lo.Y, c.Y))
				hi = geom.NewVec(max(hi.X, c.X), max(hi.Y, c.Y))
			}
			for x := int(lo.X / square); x <= int(hi.X/square); x++ {
				for y := int(lo.Y / square); y <= int(hi.Y/square); y++ {
					grid[[2]int{x, y}] = append(grid[[2]int{x, y}], l)
				}
			}
			// the lines the stretch is marked with along its edges, where it is: an arm's
			// begin past its junction's reach
			if d := geom.NewVec(p[2]-p[0], p[3]-p[1]); strip && p[12] > 0 && kind&arm == 0 &&
				run > m {
				way := geom.NewVec(d.X/run, d.Y/run)
				for k, sign := range [2]float32{-1, 1} {
					side := geom.NewVec((p[4]+p[6])/2*sign, (p[5]+p[7])/2*sign)
					w := f32.Hypot(side.X, side.Y)
					if w <= 0.4*m || int(p[19])&(k+1) != 0 {
						continue
					}
					in := (w - 0.4*m) / w
					marks = append(marks, mark{
						at:     geom.NewVec((p[0]+p[2])/2+side.X*in, (p[1]+p[3])/2+side.Y*in),
						way:    way,
						z:      (p[8] + p[9]) / 2,
						of:     owner,
						sector: sector, index: i / floats,
					})
				}
			}
			if p[15] != 0 {
				continue // under the ground by its own floor
			}
			worst, where := float32(0), geom.Vec{}
			for _, q := range l.inside() {
				z, ok := l.over(q)
				if !ok {
					continue
				}
				if d := ground(q) - z; d > worst {
					worst, where = d, q
				}
			}
			if worst > buriedBy*m {
				flaws = append(
					flaws,
					Flaw{At: where, What: Buried, Of: r.nodes[owner].of(), By: worst},
				)
			}
		}
	}
	// how high the highest of the road of owners lies at q, and whether any of it does
	over := func(q geom.Vec, owners ...int) (top float32, any bool) {
		for _, o := range owners {
			for i := range by[o] {
				if z, ok := by[o][i].over(q); ok && (!any || z > top) {
					top, any = z, true
				}
			}
		}
		return top, any
	}
	// how wide the widest stretch of the road of owner lying at q is there
	wide := func(q geom.Vec, owner int) float32 {
		w := float32(0)
		for i := range by[owner] {
			l := &by[owner][i]
			if _, ok := l.over(q); !ok || !l.strip {
				continue
			}
			d := geom.NewVec(l.p1.X-l.p0.X, l.p1.Y-l.p0.Y)
			t := ((q.X-l.p0.X)*d.X + (q.Y-l.p0.Y)*d.Y) / max(d.X*d.X+d.Y*d.Y, f32.Eps(1))
			w = max(w, 2*(l.w0+(l.w1-l.w0)*min(max(t, 0), 1)))
		}
		return w
	}
	seam := func(q, way geom.Vec, a, b int, plate bool) {
		near := 0.3 * m
		qa := geom.NewVec(q.X-way.X*near, q.Y-way.Y*near)
		qb := geom.NewVec(q.X+way.X*near, q.Y+way.Y*near)
		for _, at := range [3]geom.Vec{qa, q, qb} {
			if _, ok := over(at, a, b); !ok {
				flaws = append(
					flaws,
					Flaw{At: at, What: Hole, Of: r.nodes[a].of() + " to " + r.nodes[b].of()},
				)
				return
			}
		}
		// where the road ends at the seam its end says how high: a stretch bent sharp folds
		// over itself there
		ends := func(owner int) (float32, bool) {
			for i := range by[owner] {
				l := &by[owner][i]
				if !l.strip {
					continue
				}
				if gap(l.p0, q) < 0.05*m {
					return l.z[0], true
				}
				if gap(l.p1, q) < 0.05*m {
					return l.z[2], true
				}
			}
			return over(q, owner)
		}
		za, okA := ends(a)
		zb, okB := ends(b)
		if !okA || !okB {
			return
		}
		if d := f32.Abs(za - zb); d > stepBy*m {
			flaws = append(
				flaws,
				Flaw{At: q, What: Step, Of: r.nodes[a].of() + " to " + r.nodes[b].of(), By: d},
			)
		}
		if plate {
			return
		}
		wa, wb := wide(q, a), wide(q, b)
		if d := f32.Abs(wa - wb); wa > 0 && wb > 0 && d > narrowBy*m {
			flaws = append(
				flaws,
				Flaw{At: q, What: Narrow, Of: r.nodes[a].of() + " to " + r.nodes[b].of(), By: d},
			)
		}
	}
	// a line runs along the roadway's edge: asphalt lying a way off it on both sides, it lies
	// across a road
	whole := r.surveyed()
	lies := func(q geom.Vec, z float32) bool {
		for _, l := range whole[[2]int{int(q.X / square), int(q.Y / square)}] {
			if at, ok := l.over(q); ok && f32.Abs(at-z) < 1.5*m {
				return true
			}
		}
		return false
	}
	for _, k := range marks {
		// by a corner the roadway lies near on both sides of a short stretch
		if k.long < m {
			continue
		}
		off := geom.NewVec(-k.way.Y*(lineInset+deepBy/2)*m, k.way.X*(lineInset+deepBy/2)*m)
		if lies(geom.NewVec(k.at.X+off.X, k.at.Y+off.Y), k.z) &&
			lies(geom.NewVec(k.at.X-off.X, k.at.Y-off.Y), k.z) {
			flaws = append(flaws, Flaw{At: k.at, What: Line, Of: r.nodes[k.of].of()})
		}
	}
	// where roads meet, the roadway's top over the ground all over a piece: the ground folds
	// between the places a junction's pieces are laid at
	for sector, all := range r.pieces {
		for i := 0; i < len(all); i += floats {
			p := all[i : i+floats]
			owner, part := r.owners[sector][i/floats], r.parts[sector][i/floats]
			kind := int(p[13])
			if kind&(bank|marking) != 0 || p[15] != 0 || [3]float32(p[16:19]) == lineRGB ||
				len(r.nodes[owner].arms) < 3 && r.nodes[owner].round == 0 && part >= 0 {
				continue
			}
			l := laid{
				at: [4]geom.Vec{
					geom.NewVec(p[0]-p[4], p[1]-p[5]),
					geom.NewVec(p[0]+p[4], p[1]+p[5]),
					geom.NewVec(p[2]+p[6], p[3]+p[7]),
					geom.NewVec(p[2]-p[6], p[3]-p[7]),
				},
				z: [4]float32{p[8], p[8], p[9], p[9]},
			}
			if kind&disc != 0 {
				l.round = gap(geom.NewVec(p[0], p[1]), geom.NewVec(p[2], p[3])) / 2
			}
			worst, where := float32(0), geom.Vec{}
			look := func(q geom.Vec) {
				z, ok := l.over(q)
				if !ok {
					return
				}
				top := z
				for _, o := range whole[[2]int{int(q.X / square), int(q.Y / square)}] {
					if at, ok := o.over(q); ok && at < z+deckBy*m {
						top = max(top, at)
					}
				}
				if d := ground(q) - top; d > worst {
					worst, where = d, q
				}
			}
			for _, u := range [3]float32{0.1, 0.5, 0.9} {
				for _, v := range [3]float32{0.1, 0.5, 0.9} {
					a := geom.NewVec(l.at[0].X+(l.at[1].X-l.at[0].X)*u,
						l.at[0].Y+(l.at[1].Y-l.at[0].Y)*u)
					b := geom.NewVec(l.at[3].X+(l.at[2].X-l.at[3].X)*u,
						l.at[3].Y+(l.at[2].Y-l.at[3].Y)*u)
					look(geom.NewVec(a.X+(b.X-a.X)*v, a.Y+(b.Y-a.Y)*v))
				}
			}
			// and along its edges, a hair in, where a crack between two pieces shows
			mid := geom.NewVec((l.at[0].X+l.at[1].X+l.at[2].X+l.at[3].X)/4,
				(l.at[0].Y+l.at[1].Y+l.at[2].Y+l.at[3].Y)/4)
			for e := range 4 {
				a, b := l.at[e], l.at[(e+1)%4]
				n := int(gap(a, b)/(0.5*m)) + 1
				for k := 0; k <= n; k++ {
					t := float32(k) / float32(n)
					q := geom.NewVec(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
					if d := gap(q, mid); d > 0 {
						in := min(0.03*m, d) / d
						look(geom.NewVec(q.X+(mid.X-q.X)*in, q.Y+(mid.Y-q.Y)*in))
					}
				}
			}
			if worst > buriedBy*m {
				flaws = append(flaws, Flaw{At: where, What: Buried, Of: r.nodes[owner].of(),
					By: worst})
			}
		}
	}
	// a line lies on the roadway it is drawn over: no piece of it under the roadway there, but
	// for a deck over it
	for _, k := range marks {
		if !k.over {
			continue
		}
		p := r.pieces[k.sector][k.index*floats : (k.index+1)*floats]
		for _, t := range [5]float32{0, 0.25, 0.5, 0.75, 1} {
			q := geom.NewVec(p[0]+(p[2]-p[0])*t, p[1]+(p[3]-p[1])*t)
			z := p[8] + (p[9]-p[8])*t
			top := z
			for _, l := range whole[[2]int{int(q.X / square), int(q.Y / square)}] {
				if at, ok := l.over(q); ok && at < z+deckBy*m {
					top = max(top, at)
				}
			}
			if top > z+hiddenBy*m {
				flaws = append(flaws, Flaw{At: q, What: Hidden, Of: r.nodes[k.of].of(),
					By: top - z})
				break
			}
		}
	}
	flaws = append(flaws, r.ends()...)
	nodes := r.nodes
	for a := range nodes {
		n := &nodes[a]
		if n.sunk || n.ring != 0 && n.round == 0 {
			continue
		}
		for _, b := range n.arms {
			o := &nodes[b]
			d := geom.NewVec(o.at.X-n.at.X, o.at.Y-n.at.Y)
			l := f32.Hypot(d.X, d.Y)
			if o.sunk || l == 0 || n.round != 0 {
				continue
			}
			way := geom.NewVec(d.X/l, d.Y/l)
			if o.ring != 0 {
				// into a roundabout: from where the road ends on in to the rim
				c := o.ring - 1
				j := r.joint(nodes, a, b)
				seam(j, way, a, c, true)
				for q := j; gap(q, nodes[c].at) > nodes[c].round; {
					q = geom.NewVec(q.X+way.X*0.5*m, q.Y+way.Y*0.5*m)
					if gap(q, j) > 4*nodes[c].round+40*m {
						break
					}
					if _, ok := over(q, a, c); !ok {
						flaws = append(
							flaws,
							Flaw{At: q, What: Hole, Of: r.nodes[a].of() + " to roundabout"},
						)
						break
					}
				}
				continue
			}
			if b < a {
				continue
			}
			seam(r.joint(nodes, a, b), way, a, b, false)
		}
	}
	return flaws
}

// inside are places within l to try it at
func (l *laid) inside() []geom.Vec {
	mid := geom.NewVec((l.at[0].X+l.at[1].X+l.at[2].X+l.at[3].X)/4,
		(l.at[0].Y+l.at[1].Y+l.at[2].Y+l.at[3].Y)/4)
	out := []geom.Vec{mid}
	for _, share := range [2]float32{0.5, 0.9} {
		for _, c := range l.at {
			out = append(out, geom.NewVec(mid.X+(c.X-mid.X)*share, mid.Y+(c.Y-mid.Y)*share))
		}
	}
	return out
}

// over is how high l lies at q, false where it does not lie there
func (l *laid) over(q geom.Vec) (float32, bool) {
	if l.round > 0 {
		mid := geom.NewVec((l.at[0].X+l.at[2].X)/2, (l.at[0].Y+l.at[2].Y)/2)
		if gap(mid, q) > l.round {
			return 0, false
		}
	}
	// as the shader cuts it: from the start's right corner to the end's, and the end's left
	for _, t := range [2][3]int{{0, 3, 1}, {3, 2, 1}} {
		a, b, c := l.at[t[0]], l.at[t[1]], l.at[t[2]]
		det := (b.Y-c.Y)*(a.X-c.X) + (c.X-b.X)*(a.Y-c.Y)
		if f32.Abs(det) < f32.Eps(1) {
			continue
		}
		u := ((b.Y-c.Y)*(q.X-c.X) + (c.X-b.X)*(q.Y-c.Y)) / det
		v := ((c.Y-a.Y)*(q.X-c.X) + (a.X-c.X)*(q.Y-c.Y)) / det
		const edge = -1e-3
		if u < edge || v < edge || 1-u-v < edge {
			continue
		}
		return u*l.z[t[0]] + v*l.z[t[1]] + (1-u-v)*l.z[t[2]], true
	}
	return 0, false
}

// the squares the pieces of road are looked up by, world units
const square = 64

// surveyed are the pieces of road laid, by the squares each lies in: the roadways, no bank
// and no line
func (r *Renderer) surveyed() map[[2]int][]laid {
	grid := map[[2]int][]laid{}
	for sector, all := range r.pieces {
		for i := 0; i < len(all); i += floats {
			p := all[i : i+floats]
			kind := int(p[13])
			if kind&bank != 0 || [3]float32(p[16:19]) == lineRGB {
				continue
			}
			l := laid{
				at: [4]geom.Vec{
					geom.NewVec(p[0]-p[4], p[1]-p[5]),
					geom.NewVec(p[0]+p[4], p[1]+p[5]),
					geom.NewVec(p[2]+p[6], p[3]+p[7]),
					geom.NewVec(p[2]-p[6], p[3]-p[7]),
				},
				z:      [4]float32{p[8], p[8], p[9], p[9]},
				sector: sector,
				index:  i / floats,
				owner:  r.owners[sector][i/floats],
				part:   r.parts[sector][i/floats],
				strip:  (p[4] != 0 || p[5] != 0) && (p[6] != 0 || p[7] != 0) && kind&disc == 0,
				p0:     geom.NewVec(p[0], p[1]),
				p1:     geom.NewVec(p[2], p[3]),
			}
			if kind&disc != 0 {
				l.round = gap(l.p0, l.p1) / 2
			}
			lo, hi := l.at[0], l.at[0]
			for _, c := range l.at {
				lo = geom.NewVec(min(lo.X, c.X), min(lo.Y, c.Y))
				hi = geom.NewVec(max(hi.X, c.X), max(hi.Y, c.Y))
			}
			for x := int(lo.X / square); x <= int(hi.X/square); x++ {
				for y := int(lo.Y / square); y <= int(hi.Y/square); y++ {
					grid[[2]int{x, y}] = append(grid[[2]int{x, y}], l)
				}
			}
		}
	}
	return grid
}

// run is a stretch of line as it is drawn: from where to where, how high, and the piece it is
// drawn with
type run struct {
	a, b          geom.Vec
	za, zb        float32
	sector, index int
	dash          bool // of the line a road gives way at: broken as it is laid
}

// loose is an end of a line no other line runs on from
type loose struct {
	at, from      geom.Vec
	z             float32
	sector, index int
}

// loose are the ends of lines with no other line to run on into: every line along an edge,
// round a corner or a rim runs from one into the next, but at a road's end, a tunnel's mouth
// and the board's edge
func (r *Renderer) loose() ([]loose, []run) {
	m := r.cfg.Metre
	grid := map[[2]int][]run{}
	var runs []run
	put := func(k run) {
		runs = append(runs, k)
		ka := [2]int{int(k.a.X / square), int(k.a.Y / square)}
		kb := [2]int{int(k.b.X / square), int(k.b.Y / square)}
		grid[ka] = append(grid[ka], k)
		if kb != ka {
			grid[kb] = append(grid[kb], k)
		}
	}
	for sector, all := range r.pieces {
		for i := 0; i < len(all); i += floats {
			p := all[i : i+floats]
			kind := int(p[13])
			if kind&(bank|disc) != 0 || p[15] != 0 || p[0] == p[2] && p[1] == p[3] {
				continue
			}
			if [3]float32(p[16:19]) == lineRGB {
				if p[4] != 0 || p[5] != 0 {
					put(run{geom.NewVec(p[0], p[1]), geom.NewVec(p[2], p[3]), p[8], p[9], sector,
						i / floats, f32.Hypot(p[4], p[5]) > 1.5*lineHalf*m})
				}
				continue
			}
			if p[12] <= 0 || p[4] == 0 && p[5] == 0 || p[6] == 0 && p[7] == 0 {
				continue
			}
			for e, sign := range [2]float32{-1, 1} {
				if int(p[19])&(e+1) != 0 {
					continue
				}
				var at [2]geom.Vec
				for k := range at {
					side := geom.NewVec(p[4+2*k]*sign, p[5+2*k]*sign)
					w := f32.Hypot(side.X, side.Y)
					in := max(w-lineInset*m, 0) / w
					at[k] = geom.NewVec(p[2*k]+side.X*in, p[1+2*k]+side.Y*in)
				}
				put(run{at[0], at[1], p[8] + lineLift, p[9] + lineLift, sector, i / floats, false})
			}
		}
	}
	// how far q lies off the run from a to b
	off := func(q, a, b geom.Vec) float32 {
		d := geom.NewVec(b.X-a.X, b.Y-a.Y)
		t := ((q.X-a.X)*d.X + (q.Y-a.Y)*d.Y) / max(d.X*d.X+d.Y*d.Y, f32.Eps(1))
		t = min(max(t, 0), 1)
		return gap(q, geom.NewVec(a.X+d.X*t, a.Y+d.Y*t))
	}
	var out []loose
	for _, k := range runs {
		n := &r.nodes[r.owners[k.sector][k.index]]
		if n.of() == "end" || n.sunk || n.mouth || k.dash {
			continue
		}
		for e, q := range [2]geom.Vec{k.a, k.b} {
			from := [2]geom.Vec{k.b, k.a}[e]
			// past the board's edge nothing runs on
			l := max(gap(from, q), f32.Eps(1))
			on := geom.NewVec(q.X+(q.X-from.X)/l*10*m, q.Y+(q.Y-from.Y)/l*10*m)
			if r.off != nil && r.off(on) {
				continue
			}
			joined := false
			for dx := -1; dx <= 1 && !joined; dx++ {
				for dy := -1; dy <= 1 && !joined; dy++ {
					for _, o := range grid[[2]int{int(q.X/square) + dx, int(q.Y/square) + dy}] {
						if o.sector == k.sector && o.index == k.index {
							continue
						}
						if off(q, o.a, o.b) <= joinBy*m {
							joined = true
							break
						}
					}
				}
			}
			if !joined {
				out = append(out, loose{q, from, [2]float32{k.za, k.zb}[e], k.sector, k.index})
			}
		}
	}
	return out, runs
}

// ends are the loose ends of the lines, as flaws
func (r *Renderer) ends() []Flaw {
	var flaws []Flaw
	ends, _ := r.loose()
	for _, e := range ends {
		flaws = append(flaws, Flaw{At: e.at, What: Ends,
			Of: r.nodes[r.owners[e.sector][e.index]].of()})
	}
	return flaws
}
