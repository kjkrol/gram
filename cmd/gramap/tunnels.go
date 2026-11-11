package main

import (
	"errors"
	"maps"
	"math"
	"slices"

	"github.com/kjkrol/gram/plugins/board/cell"
	usite "github.com/kjkrol/gram/plugins/underground/site"
)

// what the tunnels are allowed, as the underground's site has it: their room and their width in
// metres of a figure's life, the rock over the roof and the ground a head's levelling moves in metres, the
// floor's rise over run, how many cells one runs under at most and at least
type tunnelsCfg struct {
	Clear   float64 `yaml:"clear"`
	Width   float64 `yaml:"width"`
	Cover   float64 `yaml:"cover"`
	Grade   float64 `yaml:"grade"`
	Longest int     `yaml:"longest"`
	Least   int     `yaml:"least"`
	Reshape float64 `yaml:"reshape"`
}

// a tunnel of the map: its heads, the cells it turns at and, of the guide's ways, those it
// stands for
type tunnel struct {
	from, to cell.ID
	via      []cell.ID
	plan     usite.Tunnel
	ends     [2][2]int // where its ways' line begins and ends, before a head drew back
}

// how many cells a head may draw back along its tunnel's line for ground closing over the
// mouth, how near another's ends a tunnel's lie to be its twin, how far off its line a tunnel's
// straight stretches may run, and how many cells in from a head no road may run over it
const (
	// how many times a tunnel's room the ground over its mouth's middle stands over its floor:
	// what is drawn of the ground there is cut a little higher than the roof
	headroom = 1.2
	drawBack = 4
	twin     = 2
	bend     = 1.5
	mouth    = 2
)

func (r *roads) tunnelRules(reshape float64) usite.Rules {
	c := r.w.r.Tunnels
	return usite.Rules{
		Longest: c.Longest,
		Cover:   float32(c.Cover / r.w.perUnit),
		Grade:   float32(c.Grade),
		Reshape: float32(reshape / r.w.perUnit),
		Width:   float32(c.Width * r.w.figures() / r.w.perUnit),
	}
}

// clear is a tunnel's room, world units
func (r *roads) clear() float32 {
	return float32(r.w.r.Tunnels.Clear * r.w.figures() / r.w.perUnit)
}

// bore digs the tunnels of ways, the longest first, and gives back the ways still roads and
// the cells a head left as it drew back, roads too
func (r *roads) bore(ways []osmWay) (left []osmWay, open [][2]int) {
	chains := r.w.chains(ways)
	slices.SortStableFunc(chains, func(a, b chain) int { return len(b.run) - len(a.run) })
	bored := map[int64]bool{}
	for _, ch := range chains {
		n := len(ch.run)
		if n < r.w.r.Tunnels.Least+2 {
			continue
		}
		ends := [2][2]int{ch.run[0], ch.run[n-1]}
		if r.twinned(ends) {
			for _, id := range ch.ways {
				bored[id] = true
			}
			continue
		}
		t, i, j, ok := r.dig(ch.run)
		if !ok {
			r.undug++
			continue
		}
		t.ends = ends
		r.tunnels = append(r.tunnels, t)
		open = append(open, ch.run[:i+1]...)
		open = append(open, ch.run[n-1-j:]...)
		for _, id := range ch.ways {
			bored[id] = true
		}
	}
	for _, w := range ways {
		if !bored[w.id] {
			left = append(left, w)
		}
	}
	return left, open
}

func (r *roads) twinned(ends [2][2]int) bool {
	near := func(a, b [2]int) bool {
		return max(abs(a[0]-b[0]), abs(a[1]-b[1])) <= twin
	}
	return slices.ContainsFunc(r.tunnels, func(t tunnel) bool {
		return near(ends[0], t.ends[0]) && near(ends[1], t.ends[1]) ||
			near(ends[0], t.ends[1]) && near(ends[1], t.ends[0])
	})
}

// dig is the tunnel under run, its heads drawn back i and j cells from run's ends, the least
// that lets it lie; its heads are laid flat and the cells it runs under noted
func (r *roads) dig(run [][2]int) (t tunnel, i, j int, ok bool) {
	n := len(run)
	for back := 0; back <= 2*drawBack; back++ {
		for i = max(0, back-drawBack); i <= min(back, drawBack); i++ {
			j = back - i
			if n-i-j < r.w.r.Tunnels.Least+2 {
				continue
			}
			if t, err := r.plan(run[i : n-j]); err == nil {
				return t, i, j, true
			}
		}
	}
	return tunnel{}, 0, 0, false
}

// plan is the tunnel from run's first cell to its last along it, or why none may lie there.
// Its heads are cut into the ground, no deeper than the tunnels' reshape, till its floor climbs
// no steeper than it may and the ground over each mouth, raised as much again at most, closes
// over the roof: a cutting and a headwall, as a mouth has.
func (r *roads) plan(run [][2]int) (tunnel, error) {
	w := r.w
	cfg := w.r.Tunnels
	n := len(run)
	t := tunnel{from: w.cell(run[0][0], run[0][1]), to: w.cell(run[n-1][0], run[n-1][1])}
	turns := [][2]int{run[0]}
	for _, v := range squared(run[0], run[n-1], straighten(run, bend)) {
		t.via = append(t.via, w.cell(v[0], v[1]))
		turns = append(turns, v)
	}
	turns = append(turns, run[n-1])
	var under []cell.ID
	for k := 1; k < len(turns); k++ {
		a, b := turns[k-1], turns[k]
		for at := a; at != b; {
			at = [2]int{at[0] + sign(b[0]-at[0]), at[1] + sign(b[1]-at[1])}
			if at != run[n-1] {
				under = append(under, w.cell(at[0], at[1]))
			}
		}
	}
	if len(under) < max(cfg.Least, 2) {
		return t, errors.New("too short for a mouth at either end")
	}
	for _, c := range under {
		if r.heads[c] {
			return t, errors.New("runs under another's head")
		}
	}
	g := boreGround{chartGround: chartGround{r: r}}
	heads := [2]cell.ID{t.from, t.to}
	firsts := [2]cell.ID{under[0], under[len(under)-1]}
	var level [2]float64
	for e, h := range heads {
		if !g.Takes(h) || r.heads[h] || r.mouths[h] {
			return t, errors.New("a head on ground no road may take")
		}
		level[e] = w.level(h)
	}
	steps := float64(len(under) + 1)
	clear := cfg.Clear * w.figures()
	// the floor under the first cell in from the head e
	floor := func(e int) float64 { return level[e] + (level[1-e]-level[e])/steps }
	// how far the ground over the mouth of the head e must rise yet
	short := func(e int) float64 {
		most := 0.0
		for k, to := range r.headwall(heads[e], firsts[e], level[e], floor(e)+headroom*clear) {
			most = max(most, to-w.corners[k])
		}
		return most
	}
	// the higher head cut down till the floor climbs as it may
	ease := func() {
		if d := math.Abs(level[1]-level[0]) - 0.98*cfg.Grade*steps*w.r.Area.Cell; d > 0 {
			if level[0] > level[1] {
				level[0] -= d
			} else {
				level[1] -= d
			}
		}
	}
	ease()
	// half of what a mouth lacks cut at the head, the other half raised over it
	for e := range heads {
		level[e] -= short(e) / 2
	}
	ease()
	var moved []height
	move := func(k int, to float64) error {
		if to == w.corners[k] {
			return nil
		}
		if math.Abs(to-w.corners[k]) > cfg.Reshape || r.wetK[k] || r.pinned[k] {
			return errors.New("a mouth on ground too uneven to shape")
		}
		moved = append(moved, height{k, w.corners[k]})
		w.corners[k] = to
		return nil
	}
	shape := func() error {
		for e, h := range heads {
			level[e] = math.Floor(level[e]*100) / 100
			for _, k := range w.cornersOf(h) {
				if err := move(k, level[e]); err != nil {
					return err
				}
			}
		}
		for e, h := range heads {
			wall := r.headwall(h, firsts[e], level[e], floor(e)+headroom*clear)
			for _, k := range slices.Sorted(maps.Keys(wall)) {
				if to := math.Ceil(wall[k]*100) / 100; to > w.corners[k] {
					if err := move(k, to); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	err := shape()
	if err == nil {
		t.plan, err = usite.Plan(g, r.tunnelRules(0), r.clear(), t.from, t.to, t.via...)
	}
	if err != nil {
		r.undo(moved)
		return t, err
	}
	r.note(t)
	return t, nil
}

// headwall is how high the corners of first, the cell a tunnel runs under first from its head,
// must stand at least for its ground's middle to lie at roof or over it, the head's corners at
// level: a corner across the cell from one of the head's, and the lower of two neither the
// head's
func (r *roads) headwall(head, first cell.ID, level, roof float64) map[int]float64 {
	w := r.w
	hs, ks := w.cornersOf(head), w.cornersOf(first)
	at := func(k int) float64 {
		if slices.Contains(hs[:], k) {
			return level
		}
		return w.corners[k]
	}
	out := map[int]float64{}
	// top-left with bottom-right, top-right with bottom-left
	for _, d := range [2][2]int{{ks[0], ks[3]}, {ks[1], ks[2]}} {
		a, b := d[0], d[1]
		if slices.Contains(hs[:], b) {
			a, b = b, a
		}
		switch {
		case slices.Contains(hs[:], b): // both the head's: a cell too short to close over
			out[a] = math.Inf(1)
		case slices.Contains(hs[:], a):
			out[b] = max(out[b], 2*roof-level)
		default:
			lo, hi := a, b
			if at(lo) > at(hi) {
				lo, hi = hi, lo
			}
			if at(hi) < roof {
				out[hi] = max(out[hi], roof)
			}
			out[lo] = max(out[lo], 2*roof-max(at(hi), roof))
		}
	}
	return out
}

// note has the tunnel t stand: its floors under its cells, its heads held, the cells by its
// mouths kept free of roads
func (r *roads) note(t tunnel) {
	g := chartGround{r: r}
	for k, c := range t.plan.Cells {
		b := r.bored[c]
		*b.Storey(t.plan.Storey) = cell.Floor{Allows: cell.Land,
			Height: t.plan.Floors[k] - g.level(c), Clear: r.clear(),
			Width: float32(r.w.deck()), Storey: t.plan.Storey}
		r.bored[c] = b
		if k < mouth || k >= len(t.plan.Cells)-mouth {
			r.mouths[c] = true
			for _, k := range r.w.cornersOf(c) {
				r.pinned[k] = true
			}
		}
	}
	for _, h := range [2]cell.ID{t.from, t.to} {
		r.heads[h] = true
		for _, k := range r.w.cornersOf(h) {
			r.pinned[k] = true
		}
	}
}

// reached keeps the tunnels a road runs to at either head; the rest, which no road of the
// guide's could be laid to, are counted and left out
func (r *roads) reached() {
	r.tunnels = slices.DeleteFunc(r.tunnels, func(t tunnel) bool {
		_, from := r.net.Node(t.from)
		_, to := r.net.Node(t.to)
		if from && to {
			return false
		}
		r.unreached++
		return true
	})
}

// held is why a tunnel would not lie under the ground as it lies, as a board holds a map's
func (r *roads) held(l *layout) error {
	g := boreGround{chartGround: chartGround{r: r, l: l}}
	for _, t := range r.tunnels {
		g.but = t.plan.Storey
		g.under = t.plan.Cells
		if _, err := usite.Plan(g, r.tunnelRules(0), r.clear(), t.from, t.to, t.via...); err != nil {
			return err
		}
	}
	return nil
}

// the ground as the underground's site asks: the tunnels dug lie under it, but for the storey
// but of the cells under, a tunnel planned again where it lies
type boreGround struct {
	chartGround
	but   uint8
	under []cell.ID
}

var _ usite.Ground = boreGround{}

func (g boreGround) Takes(c cell.ID) bool {
	return g.r.roadable(c) && !g.r.crossing(c) && !g.r.marks[c].bridged
}

func (g boreGround) Built(c cell.ID) cell.Built {
	b := g.r.bored[c]
	if g.but != 0 && slices.Contains(g.under, c) {
		*b.Storey(g.but) = cell.Floor{}
	}
	if m := g.r.marks[c]; m.bridged && !m.ford {
		b.Floor = cell.Floor{Allows: cell.Land, Height: 1}
	}
	return b
}

// a run of tunnel as OpenStreetMap has it: the ways joined end to end and the cells they pass
type chain struct {
	ways []int64
	run  [][2]int
}

// chains joins the tunnels of ways end to end where two alone meet
func (w *chart) chains(ways []osmWay) []chain {
	type end struct{ lon, lat float64 }
	at := map[end][]int{}
	var tunnels []osmWay
	for _, way := range ways {
		if !way.tunnel {
			continue
		}
		for _, p := range [2][2]float64{way.line[0], way.line[len(way.line)-1]} {
			at[end{p[0], p[1]}] = append(at[end{p[0], p[1]}], len(tunnels))
		}
		tunnels = append(tunnels, way)
	}
	used := make([]bool, len(tunnels))
	// the way joined to way i at p alone, and whether its line runs on from p
	next := func(i int, p [2]float64) (int, bool) {
		m := at[end{p[0], p[1]}]
		if len(m) != 2 {
			return 0, false
		}
		j := m[0]
		if j == i {
			j = m[1]
		}
		return j, j != i && !used[j]
	}
	var out []chain
	grow := func(i int) {
		line := slices.Clone(tunnels[i].line)
		ch := chain{ways: []int64{tunnels[i].id}}
		used[i] = true
		for _, front := range [2]bool{false, true} {
			for at := i; ; {
				p := line[len(line)-1]
				if front {
					p = line[0]
				}
				j, ok := next(at, p)
				if !ok {
					break
				}
				more := slices.Clone(tunnels[j].line)
				if more[0] != p {
					slices.Reverse(more)
				}
				if front {
					slices.Reverse(more)
					line = append(more[:len(more)-1], line...)
				} else {
					line = append(line, more[1:]...)
				}
				used[j], at = true, j
				ch.ways = append(ch.ways, tunnels[j].id)
			}
		}
		ch.run = w.trace(line)
		out = append(out, ch)
	}
	for i := range tunnels {
		if !used[i] {
			grow(i)
		}
	}
	return out
}

// straighten is the cells a line along run turns at, between its ends: run's own corners, no
// stretch between two further than bend off it, each stretch led along the grid's ways through
// one more turn where it runs along none
func straighten(run [][2]int, bend float64) [][2]int {
	keep := []int{0}
	var split func(a, b int)
	split = func(a, b int) {
		far, at := 0.0, -1
		ax, ay := float64(run[a][0]), float64(run[a][1])
		dx, dy := float64(run[b][0])-ax, float64(run[b][1])-ay
		for k := a + 1; k < b; k++ {
			px, py := float64(run[k][0])-ax, float64(run[k][1])-ay
			d := math.Hypot(px, py)
			if l := math.Hypot(dx, dy); l > 0 {
				d = math.Abs(px*dy-py*dx) / l
			}
			if d > far {
				far, at = d, k
			}
		}
		if at >= 0 && far > bend {
			split(a, at)
			split(at, b)
			return
		}
		keep = append(keep, b)
	}
	split(0, len(run)-1)
	var via [][2]int
	for k := 1; k < len(keep); k++ {
		p, q := run[keep[k-1]], run[keep[k]]
		dx, dy := q[0]-p[0], q[1]-p[1]
		if dx != 0 && dy != 0 && abs(dx) != abs(dy) {
			m := min(abs(dx), abs(dy))
			first := [2]int{p[0] + sign(dx)*m, p[1] + sign(dy)*m}
			last := [2]int{q[0] - sign(dx)*m, q[1] - sign(dy)*m}
			turn := first
			if off(run[keep[k-1]:keep[k]+1], last) < off(run[keep[k-1]:keep[k]+1], first) {
				turn = last
			}
			via = append(via, turn)
		}
		if k < len(keep)-1 {
			via = append(via, q)
		}
	}
	return via
}

// squared is via with the turns more that lead the line out of from and into to across an edge,
// as a mouth lies
func squared(from, to [2]int, via [][2]int) [][2]int {
	// the turns between p and q that leave p across an edge and run two cells on before
	// slanting: a mouth's first cell runs straight
	lead := func(p, q [2]int) [][2]int {
		dx, dy := q[0]-p[0], q[1]-p[1]
		if dx == 0 || dy == 0 {
			return nil
		}
		n := min(abs(dx), 2)
		first := [2]int{p[0] + sign(dx)*n, p[1]}
		last := [2]int{q[0], q[1] - sign(dy)*n}
		if first == last {
			return [][2]int{first}
		}
		return [][2]int{first, last}
	}
	if len(via) == 0 {
		return lead(from, to)
	}
	out := append(lead(from, via[0]), via...)
	last := lead(to, via[len(via)-1])
	slices.Reverse(last)
	return append(out, last...)
}

// off is how far p lies from the nearest cell of run
func off(run [][2]int, p [2]int) float64 {
	best := math.Inf(1)
	for _, c := range run {
		best = min(best, math.Hypot(float64(c[0]-p[0]), float64(c[1]-p[1])))
	}
	return best
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}
