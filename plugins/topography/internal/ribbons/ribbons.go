// Package ribbons draws the ways a game styles as roads, and the decks run over them, as they
// lie: a ribbon in its true width along the axes of its cells, its lanes marked, the ways
// meeting on a plate or round an island.
package ribbons

import (
	"cmp"
	"math"
	"slices"

	"embed"
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders
var shaders embed.FS

var shader = render.NewMeshShaderWith("topography ribbons",
	render.Files(shaders, "shaders/ribbon.wgsl"),
	[]render.Uniform{
		{Name: "RibbonViewProj", Size: 16},
		{Name: "RibbonEye", Size: 3},
		{Name: "RibbonBend", Size: 1},
		{Name: "RibbonHaze", Size: 4},
		{Name: "RibbonHazeWay", Size: 3},
		{Name: "RibbonSight", Size: 4},
		{Name: "RibbonMetre", Size: 1},
	}).Instanced(5)

const (
	floats = 20
	// how far over the ground a ribbon lies, and an island over its plate, world units
	lift       = 0.15
	islandLift = 0.1
	lineLift   = 0.02
	// the pieces a cell's bend is drawn in
	bends = 4
	// how often the board is read anew while its passage changes, in frames published
	every = 30
)

// the bits of a piece's kind
const (
	oneWay   = 1
	disc     = 2
	island   = 4
	arm      = 8
	backward = 16
	bank     = 32
)

// metres: how far over the ground a road's edge hangs before it is banked, the step a bank's
// foot is looked for by, how deep a bank or a wall runs into the ground, and how far out a
// wall's foot stands, for its face to have a way out
const (
	hang   = 0.3
	pace   = 0.5
	buried = 0.3
	batter = 0.02
)

// Config is what the ribbons are laid from: the board, its ground's height at a point (nil
// while it has none), how a kind of way is styled, the sector a point lies in, the view of a
// camera, how the air hazes what it sees, and how many world units a metre is.
type Config struct {
	Board  *board.BoardRead
	Ground func() func(geom.Vec) float32
	Style  func(k cell.Kind) painter.Ribbon
	Sector func(x, y float32) int
	View   func(cam camera.Camera) *view.View
	Seen   func(cam camera.Camera) (seeing, up float32, way [3]float32)
	Metre  float32
}

// Renderer draws the ribbons, a sector's those of the cells in it.
type Renderer struct {
	cfg Config

	// a sector's pieces whole, and as it is drawn far off: a cell's bend one strip, no round end
	pieces map[int][]float32
	coarse map[int][]float32
	into   map[int][]float32 // the one being laid
	solid  func(geom.Vec) bool
	plain  bool // the section being laid lies by a mouth
	// the roads being laid, those of each cell (ways, decks, sunk floors), and the one whose
	// banks are being laid
	nodes   []node
	cells   [3]map[cell.ID]int
	mine    int
	off     func(geom.Vec) bool
	under   float32 // 1 while a node under the ground is laid
	spans   map[int][2]float32
	passage uint64
	relief  uint64
	laid    bool
	wait    int
	decks   painter.Ribbon

	// the ribbons laid, kept on the GPU, and those laid since, drawn from the swap
	shown *stock
	next  *stock

	sectors []seen
	ranges  []render.InstanceRange
	opts    render.DrawMeshOptions
	own     map[string][]float32
	boxed   map[string]any
}

// stock is the pieces of every sector kept on the GPU: where each sector's lie, whole and
// coarse, and how low and how high they reach
type stock struct {
	kept   *render.Instances
	fine   map[int]render.InstanceRange
	coarse map[int]render.InstanceRange
	spans  map[int][2]float32
}

type seen struct {
	box      geom.AABB
	low, top float32
	pieces   render.InstanceRange
}

var _ render.Direct = (*Renderer)(nil)

func New(cfg Config) *Renderer {
	if cfg.Metre <= 0 {
		cfg.Metre = 1
	}
	return &Renderer{cfg: cfg}
}

// Publish lays the ribbons anew where the board's ways or the ground under them changed;
// relief counts the ground's changes.
func (r *Renderer) Publish(relief uint64) {
	ground := r.cfg.Ground()
	if ground == nil {
		return
	}
	passage := r.cfg.Board.Passage()
	if r.laid && passage == r.passage && relief == r.relief {
		return
	}
	if r.laid && r.wait > 0 {
		r.wait--
		return
	}
	r.laid, r.passage, r.relief, r.wait = true, passage, relief, every
	r.lay(ground)
	if r.next != nil {
		r.next.kept.Release()
	}
	kept, at := render.PackInstances(floats, r.pieces, r.coarse)
	r.next = &stock{kept: kept, fine: at[0], coarse: at[1], spans: r.spans}
	r.pieces, r.coarse, r.into, r.spans = nil, nil, nil, nil
}

// Swap has the ribbons laid last drawn.
func (r *Renderer) Swap() {
	if r.next == nil {
		return
	}
	if r.shown != nil {
		r.shown.kept.Release()
	}
	r.shown, r.next = r.next, nil
}

// node is a road in a cell: a way on the ground, or a deck over it
type node struct {
	c     cell.ID
	deck  bool
	sunk  bool // a floor under the ground: lit as a tunnel is
	head  bool // a deck's end lying on the ground
	mouth bool // a road on the ground running into a tunnel
	// a roundabout's: one more than the index of the roundabout whose plate it lies under,
	// the roundabout's own its own, 0 none; and the height a roundabout's plate lies at
	ring   int
	plate  float32
	tilt   geom.Vec // how much a roundabout\'s plate climbs a unit along x and along y
	at     geom.Vec
	z      float32 // a deck's level
	width  float32
	lanes  uint8
	out    cell.Links
	links  cell.Links
	round  float32 // a roundabout's radius, 0 none
	rgb    [3]float32
	isle   [3]float32
	style  painter.Ribbon
	arms   []int
	along  float32 // how far along its stretch it lies
	walked bool
}

func (r *Renderer) lay(ground func(geom.Vec) float32) {
	brd := r.cfg.Board
	// no bank lies in a solid cell, nor off the board, where nothing holds an edge either
	r.solid = func(p geom.Vec) bool {
		c, ok := brd.CellFor(p, 0, 0)
		return !ok || brd.Kind(c).Solid
	}
	r.off = func(p geom.Vec) bool {
		_, ok := brd.CellFor(p, 0, 0)
		return !ok
	}
	var nodes []node
	ways, decks, sunk := map[cell.ID]int{}, map[cell.ID]int{}, map[cell.ID]int{}
	// the level of a floor f of the cell c: its height off the mean of the ground's corners
	floor := func(c cell.ID, f cell.Floor) float32 {
		var buf [8]geom.Vec
		outline := brd.CellOutline(c, buf[:0])
		sum := float32(0)
		for _, p := range outline {
			sum += ground(p)
		}
		return sum/float32(len(outline)) + f.Height
	}
	brd.EachCell(func(c cell.ID) {
		mid := brd.CellCenter(c)
		b := brd.Built(c)
		// a deck's head lying on the ground is the road there: the way under it is not drawn
		// beside it
		head := b.Floor.Allows != 0 && b.Floor.Width > 0 && b.Floor.Links != 0 &&
			!b.Floor.Graded && !b.Floor.Sunk() && b.Floor.Height == 0
		if w := brd.Way(c); w.Runs() && !head {
			if st := r.cfg.Style(w.Kind); st != (painter.Ribbon{}) {
				n := node{c: c, at: geom.NewVec(mid.X+w.Off.X, mid.Y+w.Off.Y), width: w.Width,
					lanes: w.Lanes, out: w.Out, links: w.Links, rgb: straight(st.Surface),
					isle: straight(st.Island), style: st}
				if w.Form == cell.Roundabout {
					n.round = w.Radius
				}
				ways[c] = len(nodes)
				nodes = append(nodes, n)
				r.decks = st
			}
		}
		if f := b.Floor; f.Allows != 0 && f.Width > 0 && f.Links != 0 && !f.Graded && !f.Sunk() {
			n := node{c: c, deck: true, at: geom.NewVec(mid.X+f.Off.X, mid.Y+f.Off.Y),
				width: f.Width, lanes: f.Lanes, links: f.Links}
			// a head lies on the ground, no lower than its cell's level, which its deck's
			// body is built to
			n.z, n.head = max(ground(n.at), floor(c, f)), f.Height == 0
			if f.Height != 0 {
				n.z = floor(c, f)
			}
			decks[c] = len(nodes)
			nodes = append(nodes, n)
		}
		// a tunnel marked with a road's lanes: the road runs on along its floor, as wide as
		// its lanes of three metres and half a metre either side
		for s := uint8(0); s < cell.Storeys; s++ {
			f := *b.Storey(s)
			if !f.Sunk() || f.Lanes == 0 || f.Width <= 0 || f.Links == 0 {
				continue
			}
			wide := min(f.Width, (3*float32(f.Lanes)+1)*r.cfg.Metre)
			sunk[c] = len(nodes)
			nodes = append(nodes, node{c: c, deck: true, sunk: true, z: floor(c, f), width: wide,
				at: geom.NewVec(mid.X+f.Off.X, mid.Y+f.Off.Y), lanes: f.Lanes, links: f.Links})
		}
	})
	for i := range nodes {
		if nodes[i].deck {
			nodes[i].rgb, nodes[i].isle = straight(r.decks.Surface), straight(r.decks.Island)
			if nodes[i].lanes == 0 && !nodes[i].sunk && r.decks.Walk.A != 0 {
				nodes[i].rgb = straight(r.decks.Walk)
			}
			nodes[i].style = r.decks
		}
	}
	join := func(a, b int) {
		for _, k := range nodes[a].arms {
			if k == b {
				return
			}
		}
		nodes[a].arms = append(nodes[a].arms, b)
	}
	for i := range nodes {
		n := &nodes[i]
		// one of its own kind first: a road over a tunnel joins the road, not the tunnel
		from := [3]map[cell.ID]int{ways, decks, sunk}
		switch {
		case n.sunk:
			from = [3]map[cell.ID]int{sunk, ways, decks}
		case n.deck:
			from = [3]map[cell.ID]int{decks, ways, sunk}
		}
		for d := range brd.Directions() {
			if !n.links.Has(d) {
				continue
			}
			to, ok := brd.Toward(n.c, d)
			if !ok {
				continue
			}
			for _, m := range from {
				if k, ok := m[to]; ok {
					join(i, k)
					join(k, i)
					break
				}
			}
		}
	}
	// a deck ending where no road leads on, beside another deck's head: one road, the street
	// between them no longer than their heads
	for i := range nodes {
		n := &nodes[i]
		if !n.head || len(n.arms) != 1 {
			continue
		}
		for d := range brd.Directions() {
			to, ok := brd.Toward(n.c, d)
			if !ok {
				continue
			}
			if k, ok := decks[to]; ok && nodes[k].head && !slices.Contains(n.arms, k) {
				join(i, k)
				join(k, i)
				break
			}
		}
	}
	// three roads meeting in one another are one junction: the longest of the three ways
	// between them, which the other two already make, would close an island no car goes round
	for a := range nodes {
		if nodes[a].deck {
			continue
		}
		for _, b := range slices.Clone(nodes[a].arms) {
			for _, c := range nodes[a].arms {
				if b <= a || c == b || nodes[b].deck || nodes[c].deck ||
					!slices.Contains(nodes[b].arms, c) {
					continue
				}
				ab := gap(nodes[a].at, nodes[b].at)
				if ab > gap(nodes[a].at, nodes[c].at) && ab > gap(nodes[b].at, nodes[c].at) {
					nodes[a].arms = slices.DeleteFunc(
						nodes[a].arms,
						func(k int) bool { return k == b },
					)
					nodes[b].arms = slices.DeleteFunc(
						nodes[b].arms,
						func(k int) bool { return k == a },
					)
					break
				}
			}
		}
	}
	for i := range nodes {
		c := &nodes[i]
		if c.round == 0 {
			continue
		}
		c.ring = i + 1
		for k := range nodes {
			if n := &nodes[k]; n.round == 0 && gap(n.at, c.at) < c.round {
				n.ring = i + 1
			}
		}
		// and the cell a road runs in from, where its axis lies too near the rim for the
		// road to turn into its mouth: the mouth is laid from the cell before
		var near []int
		for k := range nodes {
			n := &nodes[k]
			if n.ring != 0 || gap(n.at, c.at) >= c.round+2*rimSet*r.cfg.Metre {
				continue
			}
			if slices.ContainsFunc(n.arms, func(a int) bool { return nodes[a].ring == i+1 }) {
				near = append(near, k)
			}
		}
		for _, k := range near {
			nodes[k].ring = i + 1
		}
		// a plane laid to the ground it and the mouths of the roads run into it cover, no
		// steeper than a road climbs, over the highest of that ground
		reach := c.round + 2*rimSet*r.cfg.Metre
		step := max(reach/10, r.cfg.Metre)
		var n, mean, xx, yy, xg, yg float32
		each := func(do func(dx, dy, g float32)) {
			for dx := -reach; dx <= reach; dx += step {
				for dy := -reach; dy <= reach; dy += step {
					if dx*dx+dy*dy <= reach*reach {
						do(dx, dy, ground(geom.NewVec(c.at.X+dx, c.at.Y+dy)))
					}
				}
			}
		}
		each(func(dx, dy, g float32) {
			n, mean, xx, yy, xg, yg = n+1, mean+g, xx+dx*dx, yy+dy*dy, xg+dx*g, yg+dy*g
		})
		mean /= n
		c.tilt = geom.NewVec(xg/xx, yg/yy)
		if steep := f32.Hypot(c.tilt.X, c.tilt.Y); steep > plateSteep {
			c.tilt = geom.NewVec(c.tilt.X/steep*plateSteep, c.tilt.Y/steep*plateSteep)
		}
		over := float32(0)
		each(func(dx, dy, g float32) { over = max(over, g-mean-c.tilt.X*dx-c.tilt.Y*dy) })
		c.plate = mean + over + lift
	}
	r.nodes, r.cells = nodes, [3]map[cell.ID]int{ways, decks, sunk}
	// a deck's head lies as a road does: over the ground under either edge
	for i := range nodes {
		n := &nodes[i]
		for _, k := range n.arms {
			n.mouth = n.mouth || !n.deck && nodes[k].sunk
		}
		if !n.head {
			continue
		}
		for _, k := range n.arms {
			d := geom.NewVec(nodes[k].at.X-n.at.X, nodes[k].at.Y-n.at.Y)
			l := f32.Hypot(d.X, d.Y)
			if l == 0 {
				continue
			}
			n.z = over(n.at, geom.NewVec(-d.Y/l*n.width/2, d.X/l*n.width/2), n.z, ground)
		}
	}
	r.walk(nodes)
	r.pieces, r.coarse = map[int][]float32{}, map[int][]float32{}
	r.spans = map[int][2]float32{}
	for i := range nodes {
		// a road run in under a roundabout's plate is the roundabout's
		if nodes[i].ring != 0 && nodes[i].round == 0 {
			continue
		}
		r.into = r.pieces
		r.piece(nodes, i, ground, bends)
		r.into = r.coarse
		r.piece(nodes, i, ground, 1)
	}
}

// walk measures every stretch between two junctions along its cells' axes
func (r *Renderer) walk(nodes []node) {
	run := func(from, first int) {
		prev, at := from, first
		far := gap(nodes[from].at, nodes[first].at)
		for len(nodes[at].arms) == 2 && !nodes[at].walked && nodes[at].round == 0 {
			nodes[at].walked, nodes[at].along = true, far
			next := nodes[at].arms[0]
			if next == prev {
				next = nodes[at].arms[1]
			}
			far += gap(nodes[at].at, nodes[next].at)
			prev, at = at, next
		}
	}
	for i := range nodes {
		if len(nodes[i].arms) != 2 || nodes[i].round > 0 {
			nodes[i].walked = true
			for _, k := range nodes[i].arms {
				run(i, k)
			}
		}
	}
	for i := range nodes {
		if !nodes[i].walked {
			nodes[i].walked = true
			run(i, nodes[i].arms[0])
		}
	}
}

func gap(a, b geom.Vec) float32 { return f32.Hypot(b.X-a.X, b.Y-a.Y) }

// piece lays the road in the cell of nodes[i]: a bend through its axis in bends strips where it
// runs through, an arm to each neighbour and a plate under them where more meet or it ends, no
// plate at an end where the bend is one strip
func (r *Renderer) piece(nodes []node, i int, ground func(geom.Vec) float32, bends int) {
	n := &nodes[i]
	sector := r.cfg.Sector(n.at.X, n.at.Y)
	r.under = 0
	if n.sunk {
		r.under = 1
	}
	lanes := float32(n.lanes)
	half := func(k int) geom.Vec { return r.joint(nodes, i, k) }
	// the height of the roundabout the road towards k runs into, where it does
	rim := func(k int) (float32, bool) {
		if n.ring == 0 && nodes[k].ring != 0 {
			return nodes[nodes[k].ring-1].level(half(k)), true
		}
		return 0, false
	}
	wide := func(k int) float32 { return halfBetween(n, &nodes[k]) }
	height := func(p geom.Vec, share float32, k int) float32 {
		if !n.deck {
			return ground(p)
		}
		to := &nodes[k]
		if !to.deck {
			// it meets the ground where the two cells meet, as a mouth's floor does; a deck
			// meets the road there, which lies over the ground under either edge
			h := half(k)
			meet := ground(h)
			if d := geom.NewVec(h.X-n.at.X, h.Y-n.at.Y); !n.sunk && d != (geom.Vec{}) {
				l := f32.Hypot(d.X, d.Y)
				meet = over(h, geom.NewVec(-d.Y/l*wide(k), d.X/l*wide(k)), meet, ground)
			}
			return n.z + (meet-n.z)*share
		}
		return n.z + (to.z-n.z)*share/2
	}
	if n.round > 0 {
		r.roundabout(nodes, i, ground, bends)
		return
	}
	if len(n.arms) >= 3 {
		r.junction(nodes, i, ground, bends)
		return
	}
	if len(n.arms) == 2 && n.round == 0 {
		a, b := n.arms[0], n.arms[1]
		// from the neighbour nearer its stretch's start
		if nodes[a].along > nodes[b].along && len(nodes[b].arms) == 2 ||
			len(nodes[a].arms) == 2 && len(nodes[b].arms) != 2 && nodes[a].along > n.along {
			a, b = b, a
		}
		p0, p2 := half(a), half(b)
		kind := float32(0)
		if n.out != 0 && n.out != n.links {
			kind = oneWay
			if to, ok := r.cfg.Board.Toward(n.c, lowest(n.out)); ok && to == nodes[a].c {
				kind += backward
			}
		}
		s0 := n.along - gap(p0, n.at)
		var last geom.Vec
		var lastN geom.Vec
		var lastZ, lastS float32
		for k := 0; k <= bends; k++ {
			t := float32(k) / float32(bends)
			p := geom.NewVec(
				(1-t)*(1-t)*p0.X+2*t*(1-t)*n.at.X+t*t*p2.X,
				(1-t)*(1-t)*p0.Y+2*t*(1-t)*n.at.Y+t*t*p2.Y)
			d := geom.NewVec(
				2*(1-t)*(n.at.X-p0.X)+2*t*(p2.X-n.at.X),
				2*(1-t)*(n.at.Y-p0.Y)+2*t*(p2.Y-n.at.Y))
			w := n.width/2 + (wide(b)-n.width/2)*(2*t-1)
			if t <= 0.5 {
				w = wide(a) + (n.width/2-wide(a))*2*t
			}
			l := max(f32.Hypot(d.X, d.Y), f32.Eps(1))
			side := geom.NewVec(-d.Y/l*w, d.X/l*w)
			var z float32
			if t <= 0.5 {
				z = height(p, 1-2*t, a)
			} else {
				z = height(p, 2*t-1, b)
			}
			r.plain = t <= 0.5 && nodes[a].sunk || t >= 0.5 && nodes[b].sunk
			z = r.level(n, p, side, z, ground)
			// down to the roundabout's plate where it runs into one
			if at, ok := rim(a); ok && t < 0.5 {
				z += (at - z) * (1 - 2*t)
			}
			if at, ok := rim(b); ok && t > 0.5 {
				z += (at - z) * (2*t - 1)
			}
			s := s0
			if k > 0 {
				s = lastS + gap(last, p)
				r.add(sector, last, p, lastN, side, lastZ, z, lastS, s, lanes, kind, n.rgb, 0)
				r.banks(i, sector, last, p, lastN, side, lastZ, z, ground)
			}
			last, lastN, lastZ, lastS = p, side, z, s
		}
		return
	}
	reach := n.width / 2 * 1.15
	for _, k := range n.arms {
		reach = max(reach, min(nodes[k].width, 2*n.width)/2*1.15)
	}
	for _, k := range n.arms {
		p := half(k)
		d := geom.NewVec(p.X-n.at.X, p.Y-n.at.Y)
		l := max(f32.Hypot(d.X, d.Y), f32.Eps(1))
		w0, w1 := n.width/2, wide(k)
		if len(n.arms) > 1 {
			w0 = min(nodes[k].width, n.width*2) / 2
		}
		n0 := geom.NewVec(-d.Y/l*w0, d.X/l*w0)
		n1 := geom.NewVec(-d.Y/l*w1, d.X/l*w1)
		r.plain = n.mouth
		z0 := r.level(n, n.at, n0, height(n.at, 0, k), ground)
		r.plain = nodes[k].sunk
		z1 := r.level(n, p, n1, height(p, 1, k), ground)
		if at, ok := rim(k); ok {
			z1 = at
		}
		r.add(sector, n.at, p, n0, n1, z0, z1, 0, l, float32(nodes[k].lanes), arm, n.rgb, reach)
		r.banks(i, sector, n.at, p, n0, n1, z0, z1, ground)
	}
	z := n.z
	if !n.deck {
		z = ground(n.at)
		for _, dx := range [2]float32{-reach, reach} {
			for _, dy := range [2]float32{-reach, reach} {
				z = max(z, ground(geom.NewVec(n.at.X+dx*0.7, n.at.Y+dy*0.7)))
			}
		}
	}
	if bends == 1 && len(n.arms) == 1 {
		return
	}
	r.round(sector, n.at, reach, z+lift, disc, n.rgb, 0, 0)
}

// metres: how far out of a roundabout's rim a road's own strip ends, its mouth from there in
// the roundabout's, the corners rounded over as much; and a dash of the line a road gives way
// at, with the gap after it
const (
	rimSet  = 3
	rimDash = 1
	// how far out a skirt's bank runs at most
	skirtReach = 15
	// how far under the plate a roundabout's verge lies, in metres
	verge = 0.1
)

// the steepest a roundabout's plate lies, rise over run
const plateSteep = 0.08

// mouthLong is how far out of a roundabout's rim a road's own strip ends: a mouth's length,
// less where the road's cell lies nearer than that or the roundabout is small
func mouthLong(metre, off, round float32) float32 {
	return min(rimSet*metre, 0.8*off, round/4)
}

// crossing is where the line from a, outside the circle r round c, on towards b first crosses
// the circle; where it passes the circle by, the circle's point nearest b
func crossing(a, b, c geom.Vec, r float32) (geom.Vec, bool) {
	d := geom.NewVec(b.X-a.X, b.Y-a.Y)
	f := geom.NewVec(a.X-c.X, a.Y-c.Y)
	qa := d.X*d.X + d.Y*d.Y
	qb := 2 * (f.X*d.X + f.Y*d.Y)
	qc := f.X*f.X + f.Y*f.Y - r*r
	if qa == 0 || qc < 0 {
		return geom.Vec{}, false
	}
	if disc := qb*qb - 4*qa*qc; disc >= 0 {
		if t := (-qb - f32.Sqrt(disc)) / (2 * qa); t >= 0 {
			return geom.NewVec(a.X+d.X*t, a.Y+d.Y*t), true
		}
	}
	to := geom.NewVec(b.X-c.X, b.Y-c.Y)
	l := f32.Hypot(to.X, to.Y)
	if l < f32.Eps(1) {
		return geom.Vec{}, false
	}
	return geom.NewVec(c.X+to.X/l*r, c.Y+to.Y/l*r), true
}

// joint is where the road in the cell of nodes[i] hands on to the one towards nodes[k]:
// halfway between their axes, or, run into a roundabout, a mouth's length short of its rim
func (r *Renderer) joint(nodes []node, i, k int) geom.Vec {
	n, to := &nodes[i], &nodes[k]
	mid := geom.NewVec((n.at.X+to.at.X)/2, (n.at.Y+to.at.Y)/2)
	if n.ring != 0 || to.ring == 0 {
		return mid
	}
	c := &nodes[to.ring-1]
	p, ok := crossing(n.at, to.at, c.at, c.round)
	if !ok {
		return mid
	}
	d := geom.NewVec(n.at.X-p.X, n.at.Y-p.Y)
	l := f32.Hypot(d.X, d.Y)
	if l < f32.Eps(1) {
		return p
	}
	set := mouthLong(r.cfg.Metre, l, c.round)
	return geom.NewVec(p.X+d.X/l*set, p.Y+d.Y/l*set)
}

// roundabout lays the roundabout of nodes[i]: its roadway a plate level round its island, a
// line along the island; where a road runs in, its mouth, the corners either side rounded
// with the road's edge line running on into the line along the rim, and across the mouth
// the broken line the road gives way at
func (r *Renderer) roundabout(nodes []node, i int, ground func(geom.Vec) float32,
	bends int) {
	c := &nodes[i]
	sector := r.cfg.Sector(c.at.X, c.at.Y)
	m := r.cfg.Metre
	ring := max(float32(c.lanes), 1) * 3.5 * m
	inner := max(c.round-ring, 0.25*c.round)
	z := c.level
	r.tilted(sector, c, c.round, 0, disc, c.rgb, 1, c.round-inner)
	r.tilted(sector, c, inner, islandLift, disc+island, c.isle, 0, 0)
	type entry struct {
		at, dir, side geom.Vec // where its axis crosses the rim, the way out, its left
		w, set        float32
		angle         float32 // round the middle: where its axis crosses
		// where each of its edges crosses the rim, its right then its left, and where its
		// edge lines cross the rim's line: the place, and the angle off its axis's
		rim, line   [2]geom.Vec
		off, offset [2]float32
		lo, hi      float32 // the angles its mouth and corners lie between
	}
	inset := lineInset * m
	// where the line through q the way dir, run in from outside, first meets the circle
	// radius round the middle; the nearest of the circle where it passes it by
	hit := func(q, dir geom.Vec, radius float32) geom.Vec {
		f := geom.NewVec(q.X-c.at.X, q.Y-c.at.Y)
		b := f.X*dir.X + f.Y*dir.Y
		if d := b*b - (f.X*f.X + f.Y*f.Y - radius*radius); d >= 0 {
			u := -b + f32.Sqrt(d)
			return geom.NewVec(q.X+dir.X*u, q.Y+dir.Y*u)
		}
		l := max(f32.Hypot(f.X, f.Y), f32.Eps(1))
		return geom.NewVec(c.at.X+f.X/l*radius, c.at.Y+f.Y/l*radius)
	}
	var entries []entry
	for j := range nodes {
		if nodes[j].ring != i+1 {
			continue
		}
		for _, k := range nodes[j].arms {
			out := &nodes[k]
			if out.ring != 0 {
				continue
			}
			p, ok := crossing(out.at, nodes[j].at, c.at, c.round)
			if !ok {
				continue
			}
			d := geom.NewVec(out.at.X-p.X, out.at.Y-p.Y)
			l := f32.Hypot(d.X, d.Y)
			if l < f32.Eps(1) {
				continue
			}
			e := entry{at: p, dir: geom.NewVec(d.X/l, d.Y/l), w: out.width / 2,
				set: mouthLong(m, l, c.round), angle: f32.Atan2(p.Y-c.at.Y, p.X-c.at.X)}
			e.side = geom.NewVec(-e.dir.Y, e.dir.X)
			for k, sign := range [2]float32{-1, 1} {
				q := geom.NewVec(p.X+sign*e.side.X*e.w, p.Y+sign*e.side.Y*e.w)
				e.rim[k] = hit(q, e.dir, c.round)
				e.off[k] = f32.Remainder(
					f32.Atan2(e.rim[k].Y-c.at.Y, e.rim[k].X-c.at.X)-e.angle, 2*math.Pi)
				q = geom.NewVec(p.X+sign*e.side.X*(e.w-inset), p.Y+sign*e.side.Y*(e.w-inset))
				e.line[k] = hit(q, e.dir, c.round-inset)
				e.offset[k] = f32.Remainder(
					f32.Atan2(e.line[k].Y-c.at.Y, e.line[k].X-c.at.X)-e.angle, 2*math.Pi)
			}
			e.lo = e.angle + e.off[0] - e.set/c.round
			e.hi = e.angle + e.off[1] + e.set/c.round
			if e.hi <= e.lo {
				continue
			}
			entries = append(entries, e)
		}
	}
	slices.SortFunc(entries, func(a, b entry) int { return cmp.Compare(a.angle, b.angle) })
	on := func(radius, angle float32) geom.Vec {
		sin, cos := f32.Sincos(angle)
		return geom.NewVec(c.at.X+cos*radius, c.at.Y+sin*radius)
	}
	paint := func(p0, p1 geom.Vec, half float32) {
		d := geom.NewVec(p1.X-p0.X, p1.Y-p0.Y)
		l := f32.Hypot(d.X, d.Y)
		if l < f32.Eps(1) {
			return
		}
		across := geom.NewVec(-d.Y/l*half, d.X/l*half)
		r.add(sector, p0, p1, across, across, z(p0)+lineLift, z(p1)+lineLift, 0, 0, 0, 0,
			lineRGB, 0)
	}
	// the line along the rim from angle from to angle to, the way the angles grow
	arc := func(from, to float32) {
		if to <= from {
			return
		}
		n := int(f32.Ceil((to-from)/0.1)) + 1
		for q := range n {
			a0 := from + (to-from)*float32(q)/float32(n)
			a1 := from + (to-from)*float32(q+1)/float32(n)
			paint(on(c.round-inset, a0), on(c.round-inset, a1), lineHalf*m)
		}
	}
	if bends == 1 {
		for _, e := range entries {
			r.mouth(sector, e.at, e.dir, e.side, e.w, e.set, min(e.set+e.w, 0.8*(c.round-inner)), z,
				c.rgb)
		}
		return
	}
	st := c.style
	r.mine = i
	// what holds the rim over lower ground: a verge as wide as the roads' mouths reach, level
	// with the plate, and a bank under it, but where roads run in
	const around = 48
	skirts := func() {
		wide := rimSet * m
		low := func(p geom.Vec) float32 { return z(p) - verge*m }
		for q := range around {
			a0 := 2 * math.Pi * float32(q) / around
			a1 := 2 * math.Pi * float32(q+1) / around
			within := func(e entry) bool {
				d := f32.Remainder((a0+a1)/2-e.angle, 2*math.Pi)
				return d > e.lo-e.angle && d < e.hi-e.angle
			}
			p, q := on(c.round+wide, a0), on(c.round+wide, a1)
			if low(p)-ground(p) <= hang*m && low(q)-ground(q) <= hang*m ||
				slices.ContainsFunc(entries, within) {
				continue
			}
			outP := geom.NewVec(p.X-c.at.X, p.Y-c.at.Y)
			outQ := geom.NewVec(q.X-c.at.X, q.Y-c.at.Y)
			in := wide / (c.round + wide)
			r.add(sector, geom.NewVec(p.X-outP.X*in, p.Y-outP.Y*in),
				geom.NewVec(q.X-outQ.X*in, q.Y-outQ.Y*in),
				geom.NewVec(outP.X*in, outP.Y*in), geom.NewVec(outQ.X*in, outQ.Y*in),
				low(p), low(q), low(p), low(q), 0, bank, straight(st.Bank), 0)
			r.skirt(st, sector, p, q, outP, outQ, low, ground)
		}
	}
	if len(entries) == 0 {
		arc(0, 2*math.Pi)
		skirts()
		return
	}
	// whether the road after entries[j] runs in so near that their corners would cross: the
	// two then share one, the point of the island between them
	joined := func(j int) bool {
		next := entries[(j+1)%len(entries)]
		room := next.lo - entries[j].hi
		if j == len(entries)-1 {
			room += 2 * math.Pi
		}
		return len(entries) > 1 && room < 0
	}
	curve := func(b [3]geom.Vec, t float32) geom.Vec {
		return geom.NewVec((1-t)*(1-t)*b[0].X+2*t*(1-t)*b[1].X+t*t*b[2].X,
			(1-t)*(1-t)*b[0].Y+2*t*(1-t)*b[1].Y+t*t*b[2].Y)
	}
	// the ground from apex out to the curve edge, the line along the curve line
	const steps = 4
	lay := func(apex geom.Vec, edge, line [3]geom.Vec) {
		for q := range steps {
			t0, t1 := float32(q)/steps, float32(q+1)/steps
			p, q := curve(edge, t0), curve(edge, t1)
			mid := geom.NewVec((p.X+q.X)/2, (p.Y+q.Y)/2)
			r.add(sector, apex, mid, geom.Vec{}, geom.NewVec((q.X-p.X)/2, (q.Y-p.Y)/2),
				z(apex), z(mid), 0, 0, 0, 0, c.rgb, 0)
			paint(curve(line, t0), curve(line, t1), lineHalf*m)
		}
	}
	dashed := float32(-math.MaxFloat32)
	for j, e := range entries {
		r.mouth(sector, e.at, e.dir, e.side, e.w, e.set, min(e.set+e.w, 0.8*(c.round-inner)), z,
			c.rgb)
		start := geom.NewVec(e.at.X+e.dir.X*e.set, e.at.Y+e.dir.Y*e.set)
		// where the road's edge on the side sign lies a mouth's length out, off in from it
		edge := func(sign, off float32) geom.Vec {
			return geom.NewVec(start.X+sign*e.side.X*(e.w-off), start.Y+sign*e.side.Y*(e.w-off))
		}
		for _, sign := range [2]float32{-1, 1} {
			if sign > 0 && joined(j) || sign < 0 && joined((j+len(entries)-1)%len(entries)) {
				continue
			}
			// the corner: from the road's edge a mouth's length out, round to the rim
			k := 0
			if sign > 0 {
				k = 1
			}
			round := sign * e.set / c.round
			at := [3]geom.Vec{edge(sign, 0), e.rim[k], on(c.round, e.angle+e.off[k]+round)}
			lay(at[1], at, [3]geom.Vec{edge(sign, inset), e.line[k],
				on(c.round-inset, e.angle+e.offset[k]+round)})
		}
		// the line it gives way at, across its mouth, on from the last road's where they meet
		from, to := max(e.angle+e.offset[0], dashed), e.angle+e.offset[1]
		dashed = to
		long := (to - from) * (c.round - inset)
		dashes := max(int(long/(2*rimDash*m)), 1)
		for q := range dashes {
			a0 := from + (to-from)*(float32(q)+0.25)/float32(dashes)
			a1 := from + (to-from)*(float32(q)+0.75)/float32(dashes)
			paint(on(c.round-inset, a0), on(c.round-inset, a1), 2*lineHalf*m)
		}
		next := entries[(j+1)%len(entries)]
		if joined(j) {
			// the point of the island between this road and the next: their edges run
			// together towards the rim
			between := (e.hi + next.lo) / 2
			if j == len(entries)-1 {
				between += math.Pi
			}
			far := geom.NewVec(next.at.X+next.dir.X*next.set, next.at.Y+next.dir.Y*next.set)
			other := func(off float32) geom.Vec {
				return geom.NewVec(far.X-next.side.X*(next.w-off), far.Y-next.side.Y*(next.w-off))
			}
			lay(on(c.round-rimSet*m, between),
				[3]geom.Vec{edge(1, 0), on(c.round, between), other(0)},
				[3]geom.Vec{edge(1, inset), on(c.round+inset, between), other(inset)})
			continue
		}
		// the rim on from this road's corner to the next road's
		end := next.lo
		if j == len(entries)-1 {
			end += 2 * math.Pi
		}
		arc(e.hi, end)
	}
	skirts()
}

// skirt lays the bank or the wall under the edge from p to q, lying as high as z says, of
// what is no road's strip: out from it the way outP and outQ point
func (r *Renderer) skirt(st painter.Ribbon, sector int, p, q, outP, outQ geom.Vec,
	z func(geom.Vec) float32, ground func(geom.Vec) float32) {
	if st.Bank.A == 0 && st.Wall.A == 0 {
		return
	}
	// a bank as wide as the edge hangs high, by one slope all the way round: feet looked for
	// one by one land apart where the ground is rough, and the bank between them is teeth
	slope := st.Slope
	if slope <= 0 {
		slope = 2.0 / 3
	}
	foot := func(e, out geom.Vec) (off geom.Vec, low float32, ok bool) {
		l := f32.Hypot(out.X, out.Y)
		h := z(e) - lift - ground(e)
		if h <= hang*r.cfg.Metre || l == 0 || r.off != nil && r.off(e) {
			return geom.Vec{}, z(e), false
		}
		far := min(h/slope, skirtReach*r.cfg.Metre)
		off = geom.NewVec(out.X/l*far, out.Y/l*far)
		at := geom.NewVec(e.X+off.X, e.Y+off.Y)
		return off, min(ground(at), ground(e)) - buried*r.cfg.Metre, true
	}
	off0, low0, ok0 := foot(p, outP)
	off1, low1, ok1 := foot(q, outQ)
	if !ok0 && !ok1 {
		return
	}
	// a skirt far out beside an edge on the ground would be a tooth
	long := gap(p, q)
	if l := f32.Hypot(off0.X, off0.Y); !ok1 && l > long {
		off0 = geom.NewVec(off0.X/l*long, off0.Y/l*long)
	}
	if l := f32.Hypot(off1.X, off1.Y); !ok0 && l > long {
		off1 = geom.NewVec(off1.X/l*long, off1.Y/l*long)
	}
	wall0, wall1 := false, false
	face := st.Bank
	if wall0 || wall1 || face.A == 0 {
		face = st.Wall
	}
	if face.A == 0 {
		face = st.Bank
	}
	r.add(sector, p, q, off0, off1, z(p), z(q), low0, low1, 0, bank, straight(face), 0)
	span := r.spans[sector]
	r.spans[sector] = [2]float32{min(span[0], low0, low1), span[1]}
}

// level is how high a roundabout's plate lies at p
func (n *node) level(p geom.Vec) float32 {
	return n.plate + n.tilt.X*(p.X-n.at.X) + n.tilt.Y*(p.Y-n.at.Y)
}

// tilted lays a roundabout's plate radius round, up over its own level: a plate whose axis
// runs the way it climbs
func (r *Renderer) tilted(sector int, c *node, radius, up, kind float32, rgb [3]float32,
	marked, wide float32) {
	way := geom.NewVec(0, 1)
	if steep := f32.Hypot(c.tilt.X, c.tilt.Y); steep > 0 {
		way = geom.NewVec(c.tilt.X/steep, c.tilt.Y/steep)
	}
	low := geom.NewVec(c.at.X-way.X*radius, c.at.Y-way.Y*radius)
	high := geom.NewVec(c.at.X+way.X*radius, c.at.Y+way.Y*radius)
	side := geom.NewVec(-way.Y*radius, way.X*radius)
	r.add(sector, low, high, side, side, c.level(low)+up, c.level(high)+up, 0, 0, marked, kind,
		rgb, wide)
}

// mouth lays the last of a road run into a roundabout: from set out of the rim, where the
// road's own strip ends, in under the plate, bare of lines
func (r *Renderer) mouth(sector int, at, dir, side geom.Vec, w, set, in float32,
	z func(geom.Vec) float32, rgb [3]float32) {
	from := geom.NewVec(at.X+dir.X*set, at.Y+dir.Y*set)
	// in under the plate as far as its edges cross the rim, however slanting it runs in,
	// short of the island
	to := geom.NewVec(at.X-dir.X*in, at.Y-dir.Y*in)
	across := geom.NewVec(side.X*w, side.Y*w)
	r.add(sector, from, to, across, across, z(from), z(to), 0, 0, 0, 0, rgb, 0)
}

func asin(x float32) float32 {
	x = min(max(x, -0.95), 0.95)
	return f32.Atan2(x, f32.Sqrt(1-x*x))
}

// halfBetween is half the width of the road from a's cell to b's where the two cells meet: the
// mean of the two where a road runs on through both, the narrower's where either is a
// junction, whose own width is its widest road's
func halfBetween(a, b *node) float32 {
	// a road run in under a roundabout keeps its own width up to the rim, whatever the ways
	// within are
	if (a.ring != 0) != (b.ring != 0) {
		if a.ring != 0 {
			return b.width / 2
		}
		return a.width / 2
	}
	if len(a.arms) >= 3 && a.round == 0 || len(b.arms) >= 3 && b.round == 0 {
		return min(a.width, b.width) / 2
	}
	return (a.width + b.width) / 4
}

// metres: how far past where two roads' edges meet a road's lines begin at a junction, the
// least a road's mouth stands off the junction's middle, how far inside its edge a road's
// edge line runs and half that line's width, as the ribbon's shader marks them
const (
	cornerRound = 2.5
	mouthLeast  = 0.5
	lineInset   = 0.4
	lineHalf    = 0.07
	// how far in from the roads' axes the asphalt closing a ring of four lies, in metres
	patchIn = 0.7
)

// the colour of a road's lines, as the ribbon's shader marks them
var lineRGB = [3]float32{0.92, 0.92, 0.9}

// junction lays the roads meeting in the cell of nodes[i]: each road up to its mouth with its
// lines, the ground between the mouths asphalt, and from each road's edge to the next road's
// a corner rounded, the edge line running on along it from one road into the other
func (r *Renderer) junction(nodes []node, i int, ground func(geom.Vec) float32, bends int) {
	n := &nodes[i]
	sector := r.cfg.Sector(n.at.X, n.at.Y)
	m := r.cfg.Metre
	type leg struct {
		k         int
		dir, side geom.Vec
		long      float32 // from the middle to where the next cell's road takes over
		w         float32 // half its width at the mouth
		set       float32 // how far off the middle its mouth lies
		angle     float32
	}
	var legs []leg
	for _, k := range n.arms {
		to := r.joint(nodes, i, k)
		d := geom.NewVec(to.X-n.at.X, to.Y-n.at.Y)
		l := f32.Hypot(d.X, d.Y)
		if l < f32.Eps(1) {
			continue
		}
		dir := geom.NewVec(d.X/l, d.Y/l)
		legs = append(legs, leg{k: k, dir: dir, side: geom.NewVec(-dir.Y, dir.X), long: l,
			w: halfBetween(n, &nodes[k]), angle: f32.Atan2(dir.Y, dir.X)})
	}
	if len(legs) < 2 {
		return
	}
	slices.SortFunc(legs, func(a, b leg) int { return cmp.Compare(a.angle, b.angle) })
	// where the edge of a towards b and the edge of b towards a meet, along a and along b;
	// ok false where they run nearly along each other
	meet := func(a, b *leg, inset float32) (t, u float32, ok bool) {
		sin := a.dir.X*b.dir.Y - a.dir.Y*b.dir.X
		cos := a.dir.X*b.dir.X + a.dir.Y*b.dir.Y
		if f32.Abs(sin) < 0.2 {
			return 0, 0, false
		}
		wa, wb := a.w-inset, b.w-inset
		return (wb + wa*cos) / sin, (wa + wb*cos) / sin, true
	}
	for j := range legs {
		a := &legs[j]
		a.set = mouthLeast * m
		for _, o := range [2]int{(j + 1) % len(legs), (j + len(legs) - 1) % len(legs)} {
			b := &legs[o]
			from, to := a, b
			if o != (j+1)%len(legs) {
				from, to = b, a
			}
			t, u, ok := meet(from, to, 0)
			if from != a {
				t = u
			}
			if ok && t > 0 {
				a.set = max(a.set, t+cornerRound*m)
			}
		}
		a.set = min(a.set, 0.9*a.long)
	}
	// level, over the ground under every mouth
	z := n.z
	if !n.deck {
		z = ground(n.at)
		for j := range legs {
			a := &legs[j]
			for _, sign := range [2]float32{-1, 1} {
				z = max(z, ground(geom.NewVec(n.at.X+a.dir.X*a.set+sign*a.side.X*a.w,
					n.at.Y+a.dir.Y*a.set+sign*a.side.Y*a.w)))
			}
		}
	}
	z += lift
	// a triangle of asphalt from the middle to the edge from p to q
	fan := func(p, q geom.Vec) {
		mid := geom.NewVec((p.X+q.X)/2, (p.Y+q.Y)/2)
		half := geom.NewVec((q.X-p.X)/2, (q.Y-p.Y)/2)
		r.add(sector, n.at, mid, geom.Vec{}, half, z, z, 0, 0, 0, 0, n.rgb, 0)
	}
	steps := 4
	if bends == 1 {
		steps = 1
	}
	for j := range legs {
		a, b := &legs[j], &legs[(j+1)%len(legs)]
		k := a.k
		// the road from its mouth on, with its lines
		mouth := geom.NewVec(n.at.X+a.dir.X*a.set, n.at.Y+a.dir.Y*a.set)
		p := geom.NewVec(n.at.X+a.dir.X*a.long, n.at.Y+a.dir.Y*a.long)
		n0 := geom.NewVec(a.side.X*a.w, a.side.Y*a.w)
		n1 := n0
		r.plain = false
		z1 := r.level(n, p, n1, n.z+(nodes[k].z-n.z)/2, ground)
		if !n.deck || !nodes[k].deck {
			at := ground(p)
			if n.deck {
				at = n.z + (over(p, n1, at, ground) - n.z)
			}
			r.plain = nodes[k].sunk
			z1 = r.level(n, p, n1, at, ground)
		}
		if n.ring == 0 && nodes[k].ring != 0 {
			z1 = nodes[nodes[k].ring-1].level(p)
		}
		kind := float32(0)
		if o := nodes[k].out; o != 0 && o != nodes[k].links && nodes[k].lanes >= 2 {
			kind = oneWay
		}
		r.add(sector, mouth, p, n0, n1, z, z1, a.set, a.long, float32(nodes[k].lanes), kind,
			n.rgb, 0)
		r.banks(i, sector, mouth, p, n0, n1, z, z1, ground)
		fan(geom.NewVec(mouth.X-n0.X, mouth.Y-n0.Y), geom.NewVec(mouth.X+n0.X, mouth.Y+n0.Y))
		// the corner on to the next road: its outline, and the edge line within it
		corner := func(inset float32) (c [3]geom.Vec) {
			wa, wb := a.w-inset, b.w-inset
			c[0] = geom.NewVec(mouth.X+a.side.X*wa, mouth.Y+a.side.Y*wa)
			c[2] = geom.NewVec(n.at.X+b.dir.X*b.set-b.side.X*wb, n.at.Y+b.dir.Y*b.set-b.side.Y*wb)
			c[1] = geom.NewVec((c[0].X+c[2].X)/2, (c[0].Y+c[2].Y)/2)
			// round where the edges meet: before both mouths where the roads close on each
			// other, or just behind them where they open away; edges meeting far off, as a
			// wide road's and a lane's nearly in line do, are joined straight
			t, u, ok := meet(a, b, inset)
			back := 2 * max(a.w, b.w)
			if ok && (t >= 0 && u >= 0 && t <= a.set && u <= b.set ||
				t <= 0 && u <= 0 && t >= -back && u >= -back) {
				c[1] = geom.NewVec(n.at.X+a.side.X*wa+a.dir.X*t, n.at.Y+a.side.Y*wa+a.dir.Y*t)
			}
			return c
		}
		at := func(c [3]geom.Vec, t float32) geom.Vec {
			return geom.NewVec((1-t)*(1-t)*c[0].X+2*t*(1-t)*c[1].X+t*t*c[2].X,
				(1-t)*(1-t)*c[0].Y+2*t*(1-t)*c[1].Y+t*t*c[2].Y)
		}
		edge, line := corner(0), corner(lineInset*m)
		for q := 0; q < steps; q++ {
			t0, t1 := float32(q)/float32(steps), float32(q+1)/float32(steps)
			fan(at(edge, t0), at(edge, t1))
		}
		if bends == 1 || nodes[k].lanes == 0 && nodes[b.k].lanes == 0 {
			continue
		}
		// two roads leaving side by side lie on each other past their mouths, as the two
		// carriageways of one do: no line runs across from one's edge to the other's
		within := func(p geom.Vec, l *leg) bool {
			d := geom.NewVec(p.X-n.at.X, p.Y-n.at.Y)
			along := d.X*l.dir.X + d.Y*l.dir.Y
			return along > 0 && along < l.long && f32.Abs(d.X*l.side.X+d.Y*l.side.Y) < l.w
		}
		if within(line[0], b) || within(line[2], a) {
			continue
		}
		// nor where their edges cross past their mouths: the corner there is a chord inside
		if t, u, ok := meet(a, b, 0); ok && t > 0 && u > 0 && (t > a.set || u > b.set) {
			continue
		}
		for q := 0; q < steps; q++ {
			t0, t1 := float32(q)/float32(steps), float32(q+1)/float32(steps)
			p0, p1 := at(line, t0), at(line, t1)
			d := geom.NewVec(p1.X-p0.X, p1.Y-p0.Y)
			l := f32.Hypot(d.X, d.Y)
			if l < f32.Eps(1) {
				continue
			}
			across := geom.NewVec(-d.Y/l*lineHalf*m, d.X/l*lineHalf*m)
			r.add(sector, p0, p1, across, across, z+lineLift, z+lineLift, 0, 0, 0, 0, lineRGB, 0)
		}
	}
	// four roads closing a ring a cell across are one junction: the ground they close is
	// asphalt, laid once, by the last of its junctions
	top := func(k int) float32 {
		w := nodes[k].width / 2
		return max(over(nodes[k].at, geom.NewVec(w, 0), ground(nodes[k].at), ground),
			over(nodes[k].at, geom.NewVec(0, w), ground(nodes[k].at), ground))
	}
	for j := range legs {
		a, b := legs[j].k, legs[(j+1)%len(legs)].k
		if len(legs) < 3 || a == b || nodes[a].deck || nodes[b].deck || n.deck {
			continue
		}
		for _, d := range nodes[a].arms {
			if d == i || nodes[d].deck || !slices.Contains(nodes[b].arms, d) {
				continue
			}
			last := true
			for _, k := range [3]int{a, b, d} {
				last = last && (k < i || len(nodes[k].arms) < 3)
			}
			if !last {
				continue
			}
			ring := [4]geom.Vec{n.at, nodes[a].at, nodes[d].at, nodes[b].at}
			mid := geom.NewVec((ring[0].X+ring[1].X+ring[2].X+ring[3].X)/4,
				(ring[0].Y+ring[1].Y+ring[2].Y+ring[3].Y)/4)
			for q := range ring {
				if l := gap(ring[q], mid); l > patchIn*m {
					ring[q] = geom.NewVec(ring[q].X+(mid.X-ring[q].X)*patchIn*m/l,
						ring[q].Y+(mid.Y-ring[q].Y)*patchIn*m/l)
				}
			}
			r.add(sector,
				geom.NewVec((ring[0].X+ring[3].X)/2, (ring[0].Y+ring[3].Y)/2),
				geom.NewVec((ring[1].X+ring[2].X)/2, (ring[1].Y+ring[2].Y)/2),
				geom.NewVec((ring[3].X-ring[0].X)/2, (ring[3].Y-ring[0].Y)/2),
				geom.NewVec((ring[2].X-ring[1].X)/2, (ring[2].Y-ring[1].Y)/2),
				max(top(i), top(b))+lift+2*lineLift, max(top(a), top(d))+lift+2*lineLift,
				0, 0, 0, 0, n.rgb, 0)
		}
	}
}

// level is the height a ribbon lies at across its width at p: level, over the ground under
// either edge there and a step along it either way, the ground folding between two places a
// road is laid at
func (r *Renderer) level(n *node, p, side geom.Vec, z float32,
	ground func(geom.Vec) float32) float32 {
	if n.sunk {
		return z + lift
	}
	// a deck lies where it is built, but never under the ground: where it leaves its head, and
	// a pier where it runs out over the shore, it keeps over the ground as a road does
	if n.deck {
		return max(z, over(p, side, ground(p), ground)) + lift
	}
	// by a mouth the ground a step on is the slope over the tunnel, which the road runs under
	if r.plain {
		return max(z, ground(geom.NewVec(p.X+side.X, p.Y+side.Y)),
			ground(geom.NewVec(p.X-side.X, p.Y-side.Y))) + lift
	}
	return over(p, side, z, ground) + lift
}

// over is z or the ground under either end of side across p, there and a step along the road
// either way, whichever is highest
func over(p, side geom.Vec, z float32, ground func(geom.Vec) float32) float32 {
	const step = 0.3
	for _, along := range [3]float32{-step, 0, step} {
		q := geom.NewVec(p.X-side.Y*along, p.Y+side.X*along)
		z = max(z, ground(geom.NewVec(q.X+side.X, q.Y+side.Y)),
			ground(geom.NewVec(q.X-side.X, q.Y-side.Y)))
	}
	return z
}

// banks lays what holds the strip from p0 to p1 over ground lying lower than its edges: at
// either side a bank from the edge down to its foot, or a wall, running out to nothing where
// the edge comes down to the ground
func (r *Renderer) banks(i, sector int, p0, p1, n0, n1 geom.Vec, z0, z1 float32,
	ground func(geom.Vec) float32) {
	n := &r.nodes[i]
	r.mine = i
	st := n.style
	if n.deck && (!n.head || n.lanes == 0) || st.Bank.A == 0 && st.Wall.A == 0 {
		return
	}
	for _, sign := range [2]float32{-1, 1} {
		e0 := geom.NewVec(p0.X+sign*n0.X, p0.Y+sign*n0.Y)
		e1 := geom.NewVec(p1.X+sign*n1.X, p1.Y+sign*n1.Y)
		off0, low0, wall0, ok0 := r.foot(st, e0, n0, sign, z0-lift, ground)
		off1, low1, wall1, ok1 := r.foot(st, e1, n1, sign, z1-lift, ground)
		if !ok0 && !ok1 {
			continue
		}
		// a bank beside a wall, or one far out beside an edge on the ground, would be a spike:
		// the stretch between them is a wall
		long := gap(e0, e1)
		sheer := func(e, side geom.Vec) (geom.Vec, float32) {
			l := f32.Hypot(side.X, side.Y)
			b := sign * batter * r.cfg.Metre / l
			return geom.NewVec(side.X*b, side.Y*b), ground(e) - buried*r.cfg.Metre
		}
		if ok0 && !wall0 && (wall1 || !ok1) && f32.Hypot(off0.X, off0.Y) > long {
			off0, low0 = sheer(e0, n0)
			wall0 = true
		}
		if ok1 && !wall1 && (wall0 || !ok0) && f32.Hypot(off1.X, off1.Y) > long {
			off1, low1 = sheer(e1, n1)
			wall1 = true
		}
		face := st.Bank
		if wall0 || wall1 || face.A == 0 {
			face = st.Wall
		}
		if face.A == 0 {
			face = st.Bank
		}
		r.add(sector, e0, e1, off0, off1, z0, z1, low0, low1, 0, bank, straight(face), 0)
		span := r.spans[sector]
		r.spans[sector] = [2]float32{min(span[0], low0, low1), span[1]}
	}
}

// foot is where the bank under a road's edge e, lying at z, comes down to the ground out along
// side by sign: how far off the edge and how high, a wall's straight under it where no bank
// lands within its reach or a solid cell stands in its way; ok false where the edge hangs
// over nothing
func (r *Renderer) foot(st painter.Ribbon, e, side geom.Vec, sign, z float32,
	ground func(geom.Vec) float32) (off geom.Vec, low float32, wall, ok bool) {
	m := r.cfg.Metre
	under := ground(e)
	l := f32.Hypot(side.X, side.Y)
	if z-under <= hang*m || l == 0 || r.off != nil && r.off(e) || r.taken(e) {
		return geom.Vec{}, z + lift, false, false
	}
	out := geom.NewVec(sign*side.X/l, sign*side.Y/l)
	slope, reach := st.Slope, st.Reach
	if slope <= 0 {
		slope = 2.0 / 3
	}
	if reach <= 0 {
		reach = 8
	}
	// no farther out than the edge's own height warrants: a bank chasing ground that falls
	// away beside a level stretch is a shard
	far := min(reach*m, 2*(z-under)/slope+m)
	if st.Bank.A != 0 {
		for d := pace * m; d <= far; d += pace * m {
			p := geom.NewVec(e.X+out.X*d, e.Y+out.Y*d)
			if r.solid != nil && r.solid(p) || r.taken(p) {
				break
			}
			if g := ground(p); z-d*slope <= g {
				return geom.NewVec(out.X*d, out.Y*d), g - buried*m, false, true
			}
		}
	}
	return geom.NewVec(out.X*batter*m, out.Y*batter*m), under - buried*m, true, true
}

// taken says another road lies at p: one of a cell round it, neither the road whose banks are
// laid nor one it runs on into, within half its width of p at its axis or halfway to a
// neighbour
func (r *Renderer) taken(p geom.Vec) bool {
	brd := r.cfg.Board
	if brd == nil {
		return false
	}
	c, ok := brd.CellFor(p, 0, 0)
	if !ok {
		return false
	}
	own := &r.nodes[r.mine]
	lies := func(c cell.ID) bool {
		for _, of := range r.cells[:2] {
			k, ok := of[c]
			if !ok || k == r.mine || slices.Contains(own.arms, k) {
				continue
			}
			m := &r.nodes[k]
			if m.deck && !m.head {
				continue
			}
			if gap(m.at, p) < m.width/2 {
				return true
			}
			for _, j := range m.arms {
				to := r.nodes[j].at
				if gap(geom.NewVec((m.at.X+to.X)/2, (m.at.Y+to.Y)/2), p) < m.width/2 {
					return true
				}
			}
		}
		return false
	}
	if lies(c) {
		return true
	}
	for d := range brd.Directions() {
		if to, ok := brd.Toward(c, d); ok && lies(to) {
			return true
		}
	}
	return false
}

func lowest(l cell.Links) int {
	for i := range 8 {
		if l.Has(i) {
			return i
		}
	}
	return 0
}

func (r *Renderer) add(sector int, p0, p1, n0, n1 geom.Vec, z0, z1, s0, s1, lanes, kind float32,
	rgb [3]float32, hide float32) {
	r.into[sector] = append(r.into[sector],
		p0.X, p0.Y, p1.X, p1.Y,
		n0.X, n0.Y, n1.X, n1.Y,
		z0, z1, s0, s1,
		lanes, kind, hide, r.under,
		rgb[0], rgb[1], rgb[2], 0)
	span, ok := r.spans[sector]
	if !ok {
		span = [2]float32{z0, z0}
	}
	r.spans[sector] = [2]float32{min(span[0], z0, z1), max(span[1], z0, z1)}
}

// round lays a plate: marked 1, a roundabout's, its roadway wide across from its rim in
func (r *Renderer) round(sector int, at geom.Vec, radius, z, kind float32, rgb [3]float32,
	marked, wide float32) {
	side := geom.NewVec(radius, 0)
	r.add(sector, geom.NewVec(at.X, at.Y-radius), geom.NewVec(at.X, at.Y+radius), side, side, z, z,
		0, 0, marked, kind, rgb, wide)
}

func straight(c color.RGBA) [3]float32 {
	return [3]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255}
}

// Compose gathers the sectors cam sees with their ribbons.
func (r *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	r.sectors = r.sectors[:0]
	st := r.shown
	if st == nil || len(st.fine) == 0 {
		return
	}
	v := r.cfg.View(cam)
	if v == nil || v.Grid() == nil {
		for k, pieces := range st.fine {
			r.sectors = append(r.sectors, seen{low: st.spans[k][0], top: st.spans[k][1],
				pieces: pieces})
		}
		slices.SortFunc(r.sectors, func(a, b seen) int {
			return a.pieces.First - b.pieces.First
		})
	} else {
		v.Sectors(func(i int, box geom.AABB, level lod.Level) {
			pieces := st.fine[i]
			if level >= lod.Strategic {
				pieces = st.coarse[i]
			}
			if pieces.Count > 0 {
				r.sectors = append(r.sectors, seen{box: box, low: st.spans[i][0],
					top: st.spans[i][1], pieces: pieces})
			}
		})
	}
	if len(r.sectors) > 0 {
		f.DrawOpaque()
	}
}

// Draw draws the ribbons gathered into the target through cam under the frame's uniforms u: of
// a picture drawn in layers, the sectors in the layer's window and not wholly in the next
// one's.
func (r *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Screen == nil || t.Depth == nil || len(r.sectors) == 0 {
		return
	}
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	field, ok := rays.Rays()
	if !ok {
		return
	}
	w, h := cam.Viewport()
	tr, ok := camera.SceneTransform(field, w, h)
	if !ok {
		return
	}
	hole, holed := camera.Inner(cam)
	r.ranges = r.ranges[:0]
	for _, s := range r.sectors {
		if s.box.BottomRight != s.box.TopLeft &&
			!tr.Shows(s.box, overhang, s.low, s.top, w, h, hole, holed) {
			continue
		}
		r.ranges = render.JoinSpan(r.ranges, s.pieces)
	}
	if len(r.ranges) == 0 {
		return
	}
	if r.own == nil {
		r.own, r.boxed, r.opts.Uniforms = map[string][]float32{}, map[string]any{}, map[string]any{}
	}
	u.Into(r.opts.Uniforms)
	r.set("RibbonViewProj", tr.M[:]...)
	r.set("RibbonEye", tr.Eye[:]...)
	r.set("RibbonBend", tr.Bend)
	seeing, up, way := r.cfg.Seen(cam)
	r.set("RibbonHaze", seeing, up, 0, 0)
	r.set("RibbonHazeWay", way[:]...)
	var fog [4]float32
	if v := r.cfg.View(cam); v != nil {
		fog = v.Fogging()
	}
	r.set("RibbonSight", fog[:]...)
	r.set("RibbonMetre", r.cfg.Metre)
	r.opts.Depth, r.opts.WriteDepth = t.Depth, true
	r.opts.Vertices, r.opts.Kept, r.opts.Spans = 6, r.shown.kept, r.ranges
	t.Screen.DrawMesh(nil, shader, &r.opts)
}

// how far past its sector's edge a ribbon may lie, world units
const overhang = 64

func (r *Renderer) set(name string, v ...float32) {
	u, ok := r.own[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		r.own[name] = u
		r.boxed[name] = u
	}
	copy(u, v)
	r.opts.Uniforms[name] = r.boxed[name]
}
