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
		{Name: "RibbonPass", Size: 1},
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
	marking  = 64 // a line laid as a piece of its own
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

// grade is how high a road lies at a place, fixed where what it meets there is not eased: a
// deck, a tunnel's floor, a roundabout's plate
type grade struct {
	z     float32
	fixed bool
}

// link is two places a stretch of road runs between, and how far apart
type link struct {
	a, b *grade
	run  float32
}

// the steepest a road is drawn, rise over run: over ground rougher than that it rides on its
// banks
const easeSteep = 0.3

func gradeAt(p geom.Vec) [2]int32 {
	return [2]int32{int32(f32.Round(p.X * 8)), int32(f32.Round(p.Y * 8))}
}

// eased is how high the road lies at p, where it would lie at z by the ground there alone
func (r *Renderer) eased(p geom.Vec, z float32, fixed bool) float32 {
	k := gradeAt(p)
	g := r.grades[k]
	if !r.measuring {
		if g != nil {
			return max(g.z, z)
		}
		return z
	}
	if g == nil {
		r.grades[k] = &grade{z: z, fixed: fixed}
		return z
	}
	g.z, g.fixed = max(g.z, z), g.fixed || fixed
	return z
}

// cleared raises the two places a stretch runs between, while they are gathered, till the
// ground between them lies under it: a ridge crossing the road between two places it is laid
// at would show through
func (r *Renderer) cleared(p0, p1, n0, n1 geom.Vec, z0, z1 float32,
	ground func(geom.Vec) float32) {
	if !r.measuring {
		return
	}
	a, b := r.grades[gradeAt(p0)], r.grades[gradeAt(p1)]
	if a == nil || b == nil || a.fixed && b.fixed {
		return
	}
	// over it all, out to its edges and its ends: the ground folds between its samples less
	// than a lane's width apart
	need := float32(0)
	for i := range 9 {
		t := float32(i) / 8
		at := geom.NewVec(p0.X+(p1.X-p0.X)*t, p0.Y+(p1.Y-p0.Y)*t)
		side := geom.NewVec(n0.X+(n1.X-n0.X)*t, n0.Y+(n1.Y-n0.Y)*t)
		for j := range 9 {
			across := float32(j)/4 - 1
			q := geom.NewVec(at.X+side.X*across, at.Y+side.Y*across)
			need = max(need, ground(q)+lift-(z0+(z1-z0)*t))
		}
	}
	if need <= 0 {
		return
	}
	// one of them fixed, the other makes up for both
	if a.fixed || b.fixed {
		need *= 2
	}
	if !a.fixed {
		a.z = max(a.z, z0+need)
	}
	if !b.fixed {
		b.z = max(b.z, z1+need)
	}
}

// ease raises the places gathered till no stretch between two climbs steeper than a road
// does, the fixed ones left where they are
func (r *Renderer) ease() {
	for range 400 {
		moved := false
		for _, l := range r.links {
			for _, e := range [2][2]*grade{{l.a, l.b}, {l.b, l.a}} {
				if least := e[1].z - easeSteep*l.run; !e[0].fixed && e[0].z < least-f32.Eps(1) {
					e[0].z, moved = least, true
				}
			}
		}
		if !moved {
			return
		}
	}
}

// Renderer draws the ribbons, a sector's those of the cells in it.
type Renderer struct {
	cfg Config

	// a sector's pieces whole, and as it is drawn far off: a cell's bend one strip, no round end
	pieces map[int][]float32
	coarse map[int][]float32
	into   map[int][]float32 // the one being laid
	// how high the roads lie where their stretches meet and bend, eased so none climbs steeper
	// than a road does, and what joins those places; measuring while they are gathered
	grades    map[[2]int32]*grade
	links     []link
	measuring bool
	// the road each piece of a sector is of and the part of it (a junction's arm, a corner,
	// a roundabout's entry; 0 its middle), those of the piece being laid, and whether the
	// roads are audited
	owners map[int][]int
	parts  map[int][]int
	cur    int
	part   int
	fine   bool
	audit  bool
	solid  func(geom.Vec) bool
	ground func(geom.Vec) float32 // the ground's height, while the roadway's edge is laid
	plain  bool                   // the section being laid lies by a mouth
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
	bare    []render.InstanceRange
	both    []render.InstanceRange
	afar    []render.InstanceRange
	opts    render.DrawMeshOptions
	own     map[string][]float32
	boxed   map[string]any
}

// stock is the pieces of every sector kept on the GPU: where each sector's lie, whole and
// coarse, and how low and how high they reach
type stock struct {
	kept   *render.Instances
	fine   map[int]render.InstanceRange
	bare   map[int]render.InstanceRange // a sector's near pieces no mark is drawn on
	coarse map[int]render.InstanceRange
	spans  map[int][2]float32
}

type seen struct {
	box      geom.AABB
	low, top float32
	pieces   render.InstanceRange
	bare     render.InstanceRange // near, the pieces the lines' pass leaves out
	far      bool                 // drawn coarse: its lines with its roadways, at one go
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
	// near, the pieces with a mark of their own apart from bare asphalt: the lines' pass draws
	// them alone
	marked, bare := map[int][]float32{}, map[int][]float32{}
	for sector, all := range r.pieces {
		for i := 0; i < len(all); i += floats {
			p := all[i : i+floats]
			if int(p[13])&marking != 0 || p[12] > 0 && int(p[19])&4 == 0 {
				marked[sector] = append(marked[sector], p...)
			} else {
				bare[sector] = append(bare[sector], p...)
			}
		}
	}
	kept, at := render.PackInstances(floats, marked, bare, r.coarse)
	r.next = &stock{kept: kept, fine: at[0], bare: at[1], coarse: at[2], spans: r.spans}
	for sector := range at[1] {
		if _, ok := r.next.fine[sector]; !ok {
			r.next.fine[sector] = render.InstanceRange{}
		}
	}
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
			st := r.cfg.Style(w.Kind)
			if r.audit && st == (painter.Ribbon{}) {
				st.Surface = color.RGBA{A: 255}
			}
			if st != (painter.Ribbon{}) {
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
		// whether the road of nodes[k] runs on into n's cell
		back := func(k int) bool {
			o := &nodes[k]
			for e := range brd.Directions() {
				if at, ok := brd.Toward(o.c, e); ok && at == n.c && o.links.Has(e) {
					return true
				}
			}
			return false
		}
		for d := range brd.Directions() {
			if !n.links.Has(d) {
				continue
			}
			to, ok := brd.Toward(n.c, d)
			if !ok {
				continue
			}
			if !n.deck {
				// the way a tunnel under a street runs out to its head is the tunnel's, not
				// the street's; a head runs on into its tunnel, not into a street over it
				_, under := sunk[to]
				s, over := sunk[n.c]
				if over && nodes[s].links.Has(d) && !under {
					continue
				}
				if k, ok := sunk[to]; ok && !over && back(k) {
					join(i, k)
					join(k, i)
					continue
				}
			}
			for _, m := range from {
				k, ok := m[to]
				// a road runs on into the road that runs on into it: another's at its own
				// height beside it is none of its road
				if !ok || !back(k) {
					continue
				}
				join(i, k)
				join(k, i)
				break
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
		reach := c.round + 2*rimSet*r.cfg.Metre
		c.plate, c.tilt = plane(c.at, reach, max(reach/10, r.cfg.Metre), ground)
		c.plate += lift + plateClear*r.cfg.Metre
	}
	r.cells = [3]map[cell.ID]int{ways, decks, sunk}
	r.build(nodes, ground)
}

// build lays the roads of nodes, joined and found under roundabouts: each cell's piece, its
// heights eased first, then the roadway's edge whole
func (r *Renderer) build(nodes []node, ground func(geom.Vec) float32) {
	r.nodes = nodes
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
	r.owners, r.parts = map[int][]int{}, map[int][]int{}
	r.grades, r.links, r.measuring = map[[2]int32]*grade{}, r.links[:0], true
	for i := range nodes {
		if nodes[i].ring == 0 || nodes[i].round != 0 {
			r.piece(nodes, i, ground, bends)
		}
	}
	r.measuring = false
	r.ease()
	for i := range nodes {
		// a road run in under a roundabout's plate is the roundabout's
		if nodes[i].ring != 0 && nodes[i].round == 0 {
			continue
		}
		r.into, r.cur, r.part, r.fine = r.pieces, i, 0, true
		r.piece(nodes, i, ground, bends)
		r.part, r.fine = 0, false
		if r.audit {
			continue
		}
		r.into = r.coarse
		r.piece(nodes, i, ground, 1)
	}
	r.outline(ground)
}

// plane is the plane laid to the ground within reach of at, measured every step: how high it
// lies at at, over the highest of that ground, and its climb either way, no steeper than a
// road climbs
func plane(at geom.Vec, reach, step float32, ground func(geom.Vec) float32) (float32, geom.Vec) {
	var n, mean, xx, yy, xg, yg float32
	// counted from the middle either way: a sum that runs out short of one side tilts the
	// plane
	k := int(reach / step)
	each := func(do func(dx, dy, g float32)) {
		for i := -k; i <= k; i++ {
			for j := -k; j <= k; j++ {
				dx, dy := float32(i)*step, float32(j)*step
				if dx*dx+dy*dy <= reach*reach {
					do(dx, dy, ground(geom.NewVec(at.X+dx, at.Y+dy)))
				}
			}
		}
	}
	each(func(dx, dy, g float32) {
		n, mean, xx, yy, xg, yg = n+1, mean+g, xx+dx*dx, yy+dy*dy, xg+dx*g, yg+dy*g
	})
	mean /= n
	var tilt geom.Vec
	if xx > 0 && yy > 0 {
		tilt = geom.NewVec(xg/xx, yg/yy)
	}
	if steep := f32.Hypot(tilt.X, tilt.Y); steep > plateSteep {
		tilt = geom.NewVec(tilt.X/steep*plateSteep, tilt.Y/steep*plateSteep)
	}
	over := float32(0)
	each(func(dx, dy, g float32) { over = max(over, g-mean-tilt.X*dx-tilt.Y*dy) })
	return mean + over, tilt
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
	// what the road meets towards k is laid by its own heights
	fixed := func(k int) bool {
		return nodes[k].deck && !nodes[k].head || nodes[k].sunk ||
			n.ring == 0 && nodes[k].ring != 0
	}
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
			// up to the roundabout's plate where it runs into one that lies higher, and onto
			// it at its end; never down under its own ground on the way
			if at, ok := rim(a); ok && t < 0.5 {
				z = max(z, z+(at-z)*(1-2*t))
				if k == 0 {
					z = at
				}
			}
			if at, ok := rim(b); ok && t > 0.5 {
				z = max(z, z+(at-z)*(2*t-1))
				if k == bends {
					z = at
				}
			}
			if !n.deck || n.head {
				z = r.eased(p, z, k == 0 && fixed(a) || k == bends && fixed(b))
			}
			s := s0
			if k > 0 {
				s = lastS + gap(last, p)
				r.cleared(last, p, lastN, side, lastZ, z, ground)
				r.add(sector, last, p, lastN, side, lastZ, z, lastS, s, lanes, kind, n.rgb, 0)
				r.banks(i, sector, last, p, lastN, side, lastZ, z, ground)
			}
			last, lastN, lastZ, lastS = p, side, z, s
		}
		return
	}
	reach := n.width / 2
	for _, k := range n.arms {
		reach = max(reach, min(nodes[k].width, 2*n.width)/2)
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
		if !n.deck || n.head {
			z0, z1 = r.eased(n.at, z0, false), r.eased(p, z1, fixed(k))
		}
		r.cleared(n.at, p, n0, n1, z0, z1, ground)
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
	// a road's end is rounded on the slope its road comes in by, not a slab level over it
	if len(n.arms) == 1 && !n.deck {
		p := half(n.arms[0])
		d := geom.NewVec(p.X-n.at.X, p.Y-n.at.Y)
		if l := f32.Hypot(d.X, d.Y); l > 0 {
			r.plain = n.mouth
			z0 := r.eased(n.at, r.level(n, n.at, geom.NewVec(-d.Y/l*n.width/2, d.X/l*n.width/2),
				ground(n.at), ground), false)
			z1 := z0
			if g := r.grades[gradeAt(p)]; g != nil {
				z1 = g.z
			}
			cap := *n
			cap.plate = z0
			cap.tilt = geom.NewVec(d.X/l*(z1-z0)/l, d.Y/l*(z1-z0)/l)
			r.tilted(sector, &cap, reach, 0, disc, n.rgb, 0, 0)
			return
		}
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
	// how far over the highest of its ground a roundabout's plate lies, in metres: the ground
	// between the places it was measured at stays under it
	plateClear = 0.06
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
	if n.ring != 0 {
		return mid
	}
	if to.ring == 0 {
		// out of a junction a road runs on from where it lies clear of the junction's other
		// roads, the junction's own roadway one surface till there
		d := gap(n.at, to.at)
		a, b := min(clearOf(nodes, i, k), 0.75*d), min(clearOf(nodes, k, i), 0.75*d)
		at := min(max(d/2, a), d-b)
		if a+b > d {
			at = d * a / (a + b)
		}
		if d < f32.Eps(1) || at == d/2 {
			return mid
		}
		return geom.NewVec(n.at.X+(to.at.X-n.at.X)*at/d, n.at.Y+(to.at.Y-n.at.Y)*at/d)
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

// clearOf is how far out of the junction nodes[i] its road towards nodes[k] lies clear of
// every other road meeting there, edge from edge; none out of a node no junction
func clearOf(nodes []node, i, k int) float32 {
	n := &nodes[i]
	if len(n.arms) < 3 || n.round != 0 {
		return 0
	}
	way := func(j int) geom.Vec {
		d := geom.NewVec(nodes[j].at.X-n.at.X, nodes[j].at.Y-n.at.Y)
		l := max(f32.Hypot(d.X, d.Y), f32.Eps(1))
		return geom.NewVec(d.X/l, d.Y/l)
	}
	u, w := way(k), halfBetween(n, &nodes[k])
	out := float32(0)
	for _, j := range n.arms {
		if j == k {
			continue
		}
		v := way(j)
		cos := u.X*v.X + u.Y*v.Y
		sin := f32.Sqrt(max(1-cos*cos, 0))
		if sin < 0.05 {
			continue
		}
		out = max(out, (halfBetween(n, &nodes[j])+w*cos)/sin)
	}
	return out
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
		// the angles off its axis's where its edges cross the rim, its right then its left
		off    [2]float32
		lo, hi float32 // the angles its mouth lies between
	}
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
				at := hit(q, e.dir, c.round)
				e.off[k] = f32.Remainder(f32.Atan2(at.Y-c.at.Y, at.X-c.at.X)-e.angle, 2*math.Pi)
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
	if bends == 1 {
		for _, e := range entries {
			r.mouth(sector, e.at, e.dir, e.side, e.w, e.set, min(e.set+e.w, 0.8*(c.round-inner)), z,
				c.rgb)
		}
		return
	}
	st := c.style
	r.mine = i
	for j, e := range entries {
		r.part = j + 1
		r.mouth(sector, e.at, e.dir, e.side, e.w, e.set, min(e.set+e.w, 0.8*(c.round-inner)), z,
			c.rgb)
	}
	r.part = 0
	// what holds the rim over lower ground: a verge as wide as the roads' mouths reach, level
	// with the plate, and a bank under it, but where roads run in
	const around = 48
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
		p0 := geom.NewVec(p.X-outP.X*in, p.Y-outP.Y*in)
		q0 := geom.NewVec(q.X-outQ.X*in, q.Y-outQ.Y*in)
		r.add(sector, p0, q0, geom.NewVec(outP.X*in, outP.Y*in),
			geom.NewVec(outQ.X*in, outQ.Y*in), low(p0), low(q0), low(p), low(q), 0, bank,
			straight(st.Bank), 0)
		r.skirt(st, sector, p, q, outP, outQ, low, ground)
	}
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
	a, b := geom.NewVec(
		from.X-across.X,
		from.Y-across.Y,
	), geom.NewVec(
		from.X+across.X,
		from.Y+across.Y,
	)
	c, d := geom.NewVec(to.X-across.X, to.Y-across.Y), geom.NewVec(to.X+across.X, to.Y+across.Y)
	// level across either end, one strip holds it
	if f32.Abs(z(a)-z(b)) < f32.Eps(1) && f32.Abs(z(c)-z(d)) < f32.Eps(1) {
		r.add(sector, from, to, across, across, z(from), z(to), 0, 0, 0, 0, rgb, 0)
		return
	}
	r.sloped(sector, a, b, c, z, rgb)
	r.sloped(sector, b, c, d, z, rgb)
}

// sloped lays the triangle a, b, c of a ground lying at z
func (r *Renderer) sloped(sector int, a, b, c geom.Vec, z func(geom.Vec) float32,
	rgb [3]float32) {
	r.tri(sector, [3]geom.Vec{a, b, c}, [3]float32{z(a), z(b), z(c)}, rgb)
}

// tri lays the triangle of the corners at, each at its height h: a strip is level across
// either end, so the triangle is cut along the level through its middle corner
func (r *Renderer) tri(sector int, at [3]geom.Vec, h [3]float32, rgb [3]float32) {
	k := [3]int{0, 1, 2}
	slices.SortFunc(k[:], func(p, q int) int { return cmp.Compare(h[p], h[q]) })
	low, mid, high := at[k[0]], at[k[1]], at[k[2]]
	zl, zm, zh := h[k[0]], h[k[1]], h[k[2]]
	x := mid
	if rise := zh - zl; rise > f32.Eps(1) {
		share := (zm - zl) / rise
		x = geom.NewVec(low.X+(high.X-low.X)*share, low.Y+(high.Y-low.Y)*share)
	}
	edge := geom.NewVec((mid.X+x.X)/2, (mid.Y+x.Y)/2)
	half := geom.NewVec((x.X-mid.X)/2, (x.Y-mid.Y)/2)
	if half == (geom.Vec{}) {
		// level: any corner against the other two
		edge = geom.NewVec((mid.X+high.X)/2, (mid.Y+high.Y)/2)
		half = geom.NewVec((high.X-mid.X)/2, (high.Y-mid.Y)/2)
		r.add(sector, low, edge, geom.Vec{}, half, zl, zm, 0, 0, 0, 0, rgb, 0)
		return
	}
	r.add(sector, low, edge, geom.Vec{}, half, zl, zm, 0, 0, 0, 0, rgb, 0)
	r.add(sector, high, edge, geom.Vec{}, half, zh, zm, 0, 0, 0, 0, rgb, 0)
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

// metres: how far inside its edge a road's edge line runs and half that line's width, as
// the ribbon's shader marks them
const (
	lineInset = 0.4
	lineHalf  = 0.07
)

// the colour of a road's lines, as the ribbon's shader marks them
var lineRGB = [3]float32{0.92, 0.92, 0.9}

// junction lays the roads meeting in the cell of nodes[i]: each from the middle to where the
// next cell's takes over, as wide as that road, on a plate as wide as the widest; the corners
// between them are rounded with the roadway whole, as its lines are laid
func (r *Renderer) junction(nodes []node, i int, ground func(geom.Vec) float32, bends int) {
	n := &nodes[i]
	sector := r.cfg.Sector(n.at.X, n.at.Y)
	// a deck lies level; a deck's head lies on the ground, as a road does
	laid := !n.deck || n.head
	reach := float32(0)
	for _, k := range n.arms {
		reach = max(reach, halfBetween(n, &nodes[k]))
	}
	z := n.z + lift
	if laid && n.mouth {
		// a tunnel's head lies level in its cutting, as the tunnel's floor runs out of it: the
		// slopes of the cutting and the headwall over the mouth rise beside and past its plate
		z = r.eased(n.at, ground(n.at)+lift, true)
	} else if laid {
		z = ground(n.at) + lift
		if n.deck {
			z = max(z, n.z+lift)
		}
		for _, k := range n.arms {
			to := r.joint(nodes, i, k)
			d := geom.NewVec(to.X-n.at.X, to.Y-n.at.Y)
			l := f32.Hypot(d.X, d.Y)
			if l < f32.Eps(1) {
				continue
			}
			w := halfBetween(n, &nodes[k])
			z = max(z, r.level(n, n.at, geom.NewVec(-d.Y/l*w, d.X/l*w), ground(n.at), ground))
		}
		// over the ground under its plate
		for q := range 8 {
			sin, cos := f32.Sincos(2 * math.Pi * float32(q) / 8)
			z = max(z, ground(geom.NewVec(n.at.X+cos*reach, n.at.Y+sin*reach))+lift)
		}
		z = r.eased(n.at, z, false)
	}
	// near, one roadway from the middle out, every road met as it runs on from there
	whole := r.fine && !r.measuring
	var arms []met
	for _, k := range n.arms {
		r.part = k + 1
		p := r.joint(nodes, i, k)
		d := geom.NewVec(p.X-n.at.X, p.Y-n.at.Y)
		l := f32.Hypot(d.X, d.Y)
		if l < f32.Eps(1) {
			continue
		}
		w := halfBetween(n, &nodes[k])
		side := geom.NewVec(-d.Y/l*w, d.X/l*w)
		r.plain = false
		z1 := r.level(n, p, side, n.z+(nodes[k].z-n.z)/2, ground)
		if !n.deck || !nodes[k].deck {
			at := ground(p)
			if n.deck {
				at = n.z + (over(p, side, at, ground) - n.z)
			}
			r.plain = nodes[k].sunk
			z1 = r.level(n, p, side, at, ground)
		}
		if n.ring == 0 && nodes[k].ring != 0 {
			z1 = nodes[nodes[k].ring-1].level(p)
		}
		if laid {
			z1 = r.eased(p, z1, nodes[k].deck && !nodes[k].head || nodes[k].sunk ||
				n.ring == 0 && nodes[k].ring != 0)
		}
		if whole {
			arms = append(arms, met{k: k, joint: p, way: geom.NewVec(d.X/l, d.Y/l), long: l,
				half: w, z: z1})
			continue
		}
		kind := float32(0)
		if o := nodes[k].out; o != 0 && o != nodes[k].links && nodes[k].lanes >= 2 {
			kind = oneWay
		}
		r.cleared(n.at, p, side, side, z, z1, ground)
		r.add(sector, n.at, p, side, side, z, z1, 0, l, float32(nodes[k].lanes), kind, n.rgb, 0)
		r.banks(i, sector, n.at, p, side, side, z, z1, ground)
	}
	r.part = 0
	if whole {
		r.fan(sector, n.at, arms, reach, z, n.rgb, ground)
		return
	}
	r.round(sector, n.at, reach, z, disc, n.rgb, 0, 0)
}

// met is a road met at a junction: its node, where it meets the road running on, the way
// out to there and how far, half its width, and how high it lies there
type met struct {
	k             int
	joint, way    geom.Vec
	long, half, z float32
}

// how many corners a junction's plate is drawn with round its middle where no road leaves it
const fanRound = 16

// fan lays a junction's roadway as one surface from its middle out to the rim of its plate,
// reach round, and to each road's joint: within a road's width level across as the road runs
// on from its joint, between two roads their heights blended round the middle
func (r *Renderer) fan(sector int, c geom.Vec, arms []met, reach, z float32, rgb [3]float32,
	ground func(geom.Vec) float32) {
	if len(arms) == 0 {
		return
	}
	turn := float32(2 * math.Pi)
	norm := func(a float32) float32 {
		for a < 0 {
			a += turn
		}
		for a >= turn {
			a -= turn
		}
		return a
	}
	var angles []float32
	for q := range fanRound {
		angles = append(angles, turn*float32(q)/fanRound)
	}
	for _, a := range arms {
		base := f32.Atan2(a.way.Y, a.way.X)
		left := geom.NewVec(-a.way.Y, a.way.X)
		for _, sign := range [2]float32{-1, 1} {
			corner := geom.NewVec(a.joint.X+left.X*a.half*sign-c.X,
				a.joint.Y+left.Y*a.half*sign-c.Y)
			angles = append(angles, norm(f32.Atan2(corner.Y, corner.X)))
			if a.half < reach {
				// where its side meets the plate's rim
				angles = append(angles, norm(base+sign*f32.Atan2(a.half,
					f32.Sqrt(reach*reach-a.half*a.half))))
			}
		}
	}
	slices.Sort(angles)
	angles = slices.CompactFunc(angles, func(a, b float32) bool { return f32.Abs(b-a) < 1e-3 })
	if len(angles) > 1 && angles[len(angles)-1]-angles[0] > turn-1e-3 {
		angles = angles[:len(angles)-1]
	}
	// the rim along each angle and its height there: a road's, level across as it runs on,
	// where it lies within the road's width; else none yet
	rim := make([]geom.Vec, len(angles))
	h := make([]float32, len(angles))
	of := make([]int, len(angles))
	for j, t := range angles {
		sin, cos := f32.Sincos(t)
		v := geom.NewVec(cos, sin)
		// out to the plate's rim, but where a road runs on from its joint: that is the next
		// cell's
		far, plate := float32(0), true
		for _, a := range arms {
			along := v.X*a.way.X + v.Y*a.way.Y
			if along <= 1e-6 {
				continue
			}
			out := a.long / along
			across := f32.Abs(v.X*-a.way.Y + v.Y*a.way.X)
			if across*a.long > a.half*along*1.001 {
				out = min(out, a.half/across)
			} else {
				plate = false
			}
			far = max(far, out)
		}
		if plate {
			far = max(far, reach)
		}
		q := geom.NewVec(c.X+v.X*far, c.Y+v.Y*far)
		rim[j], of[j], h[j] = q, -1, float32(math.NaN())
		best := float32(math.Inf(1))
		for ai, a := range arms {
			d := geom.NewVec(q.X-c.X, q.Y-c.Y)
			s := d.X*a.way.X + d.Y*a.way.Y
			off := f32.Abs(d.X*-a.way.Y + d.Y*a.way.X)
			if s < -1e-3*r.cfg.Metre || off > a.half*1.001 || off/a.half >= best {
				continue
			}
			best, of[j] = off/a.half, ai
			h[j] = z + (a.z-z)*max(s, 0)/a.long
			// over the ground along its sides; where it meets its road, as the road lies
			if s < a.long-1e-3*r.cfg.Metre {
				h[j] = max(h[j], ground(q)+lift)
			}
		}
	}
	// between roads, round the middle from one road's height to the next one's, over the
	// ground
	for j := range h {
		if of[j] >= 0 {
			continue
		}
		back, ahead := j, j
		for range len(h) {
			back = (back + len(h) - 1) % len(h)
			if of[back] >= 0 {
				break
			}
		}
		for range len(h) {
			ahead = (ahead + 1) % len(h)
			if of[ahead] >= 0 {
				break
			}
		}
		a0, a1 := angles[back], angles[ahead]
		span := norm(a1 - a0)
		share := float32(0.5)
		if span > 1e-6 {
			share = norm(angles[j]-a0) / span
		}
		at := z
		if of[back] >= 0 {
			at = h[back] + (h[ahead]-h[back])*share
		}
		h[j] = max(at, ground(rim[j])+lift)
	}
	// its middle raised till the roadway lies over the ground halfway out and in the middle of
	// each slice: a junction on a crest rises over it as a dome
	for j := range rim {
		k := (j + 1) % len(rim)
		half := geom.NewVec((c.X+rim[j].X)/2, (c.Y+rim[j].Y)/2)
		z = max(z, 2*(ground(half)+lift)-h[j])
		mid := geom.NewVec((c.X+rim[j].X+rim[k].X)/3, (c.Y+rim[j].Y+rim[k].Y)/3)
		z = max(z, 3*(ground(mid)+lift)-h[j]-h[k])
	}
	// halfway out along each ray, high enough for the roadway to lie over the ground along the
	// ray and in each third of the two slices beside it: a slice whose ray is raised is laid
	// in three
	m := r.cfg.Metre
	mid := make([]geom.Vec, len(rim))
	hm := make([]float32, len(rim))
	for j := range rim {
		mid[j] = geom.NewVec((c.X+rim[j].X)/2, (c.Y+rim[j].Y)/2)
		at := func(f float32) float32 {
			return ground(geom.NewVec(c.X+(rim[j].X-c.X)*f, c.Y+(rim[j].Y-c.Y)*f)) + lift
		}
		hm[j] = max((z+h[j])/2, at(0.5), 2*at(0.25)-z, 2*at(0.75)-h[j])
	}
	need := func(a, b, d geom.Vec, ha, hb, hd float32) float32 {
		q := geom.NewVec((a.X+b.X+d.X)/3, (a.Y+b.Y+d.Y)/3)
		return ground(q) + lift - (ha+hb+hd)/3
	}
	for range 2 {
		for j := range rim {
			k := (j + 1) % len(rim)
			if d := need(c, mid[j], mid[k], z, hm[j], hm[k]); d > 0 {
				hm[j], hm[k] = hm[j]+1.5*d, hm[k]+1.5*d
			}
			if d := need(mid[j], rim[j], rim[k], hm[j], h[j], h[k]); d > 0 {
				hm[j] += 3 * d
			}
			if d := need(mid[j], rim[k], mid[k], hm[j], h[k], hm[k]); d > 0 {
				hm[j], hm[k] = hm[j]+1.5*d, hm[k]+1.5*d
			}
		}
	}
	for j := range rim {
		k := (j + 1) % len(rim)
		r.part = 0
		if of[j] >= 0 && of[j] == of[k] {
			r.part = arms[of[j]].k + 1
		}
		if hm[j] <= (z+h[j])/2+0.01*m && hm[k] <= (z+h[k])/2+0.01*m {
			r.tri(sector, [3]geom.Vec{c, rim[j], rim[k]}, [3]float32{z, h[j], h[k]}, rgb)
			continue
		}
		r.tri(sector, [3]geom.Vec{c, mid[j], mid[k]}, [3]float32{z, hm[j], hm[k]}, rgb)
		r.tri(sector, [3]geom.Vec{mid[j], rim[j], rim[k]}, [3]float32{hm[j], h[j], h[k]}, rgb)
		r.tri(sector, [3]geom.Vec{mid[j], rim[k], mid[k]}, [3]float32{hm[j], h[k], hm[k]}, rgb)
	}
	r.part = 0
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

// over is z or the ground under side across p, at either end and in the middle, there and
// two steps along the road either way, whichever is highest: a ridge crossing the road
// between two places it is laid at shows through it
func over(p, side geom.Vec, z float32, ground func(geom.Vec) float32) float32 {
	const step = 0.35
	for _, along := range [5]float32{-2 * step, -step, 0, step, 2 * step} {
		q := geom.NewVec(p.X-side.Y*along, p.Y+side.X*along)
		z = max(z, ground(q), ground(geom.NewVec(q.X+side.X, q.Y+side.Y)),
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
	// near, the roadway's edge whole carries the banks, as its line
	if r.fine || n.deck && (!n.head || n.lanes == 0) || st.Bank.A == 0 && st.Wall.A == 0 {
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
	if r.measuring {
		a, b := r.grades[gradeAt(p0)], r.grades[gradeAt(p1)]
		if run := gap(p0, p1); a != nil && b != nil && a != b && int(kind)&bank == 0 && run > 0 {
			r.links = append(r.links, link{a, b, run})
		}
		return
	}
	if rgb == lineRGB {
		kind += marking
	}
	if r.fine {
		r.owners[sector] = append(r.owners[sector], r.cur)
		r.parts[sector] = append(r.parts[sector], r.part)
	}
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
				pieces: pieces, bare: st.bare[k]})
		}
		slices.SortFunc(r.sectors, func(a, b seen) int {
			return a.pieces.First - b.pieces.First
		})
	} else {
		v.Sectors(func(i int, box geom.AABB, level lod.Level) {
			pieces, bare, far := st.fine[i], st.bare[i], level >= lod.Strategic
			if far {
				pieces, bare = st.coarse[i], render.InstanceRange{}
			}
			if pieces.Count > 0 || bare.Count > 0 {
				r.sectors = append(r.sectors, seen{box: box, low: st.spans[i][0],
					top: st.spans[i][1], pieces: pieces, bare: bare, far: far})
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
	r.ranges, r.afar, r.bare = r.ranges[:0], r.afar[:0], r.bare[:0]
	for _, s := range r.sectors {
		if s.box.BottomRight != s.box.TopLeft &&
			!tr.Shows(s.box, overhang, s.low, s.top, w, h, hole, holed) {
			continue
		}
		if s.far {
			r.afar = render.JoinSpan(r.afar, s.pieces)
			continue
		}
		if s.pieces.Count > 0 {
			r.ranges = render.JoinSpan(r.ranges, s.pieces)
		}
		if s.bare.Count > 0 {
			r.bare = render.JoinSpan(r.bare, s.bare)
		}
	}
	if len(r.ranges) == 0 && len(r.afar) == 0 && len(r.bare) == 0 {
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
	r.opts.Vertices, r.opts.Kept = 6, r.shown.kept
	// the roadways, then their lines over them all: a line drawn with its own roadway lay
	// under the next one's wherever two met
	// far off a road is drawn coarse and its lines with it, at one go: they are too thin there
	// for one to show under another road
	if len(r.afar) > 0 {
		r.opts.Spans = r.afar
		r.set("RibbonPass", -1)
		t.Screen.DrawMesh(nil, shader, &r.opts)
	}
	if len(r.ranges) > 0 || len(r.bare) > 0 {
		r.both = append(append(r.both[:0], r.ranges...), r.bare...)
		r.opts.Spans = r.both
		r.set("RibbonPass", 0)
		t.Screen.DrawMesh(nil, shader, &r.opts)
	}
	if len(r.ranges) > 0 {
		r.opts.Spans = r.ranges
		r.set("RibbonPass", 1.5*lineLift)
		t.Screen.DrawMesh(nil, shader, &r.opts)
	}
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
