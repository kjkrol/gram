package site

import (
	"fmt"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
)

// Ground is what a bridge is planned over: a board and its heights.
type Ground interface {
	Grid() grid.Grid
	// Heights are the ground's heights at c's outline, in CellOutline's order; on a grid of level
	// cells one, its level.
	Heights(c cell.ID, dst []float32) []float32
	// Takes reports whether a bridge's head may stand on c: ground its road may take.
	Takes(c cell.ID) bool
	// Wet reports whether water runs under c.
	Wet(c cell.ID) bool
	// Bridged is the part of a bridge standing on c, if one does.
	Bridged(c cell.ID) (Part, bool)
	// Road is the links of the road running through c; none for none.
	Road(c cell.ID) cell.Links
}

// Part is what of a bridge stands on a cell: a span, or a Head, running Way.
type Part struct {
	Way  int
	Head bool
}

// Rules are where a bridge may stand: Longest cells at most between its heads, the deck
// Clearance over the ground under every one and no more than High over it (0: any), a head's
// ground moved no more than Reshape to lie flat at the deck, something Under the deck crossed.
type Rules struct {
	Longest   int
	Under     float32
	Clearance float32
	Reshape   float32
	High      float32
	Heights   bool
}

// Bridge is a bridge planned from one head to another: the way it runs from From, its deck's
// height, the cells it spans and where its piers stand.
type Bridge struct {
	From, To cell.ID
	Way      int
	Deck     float32
	Spans    []cell.ID
	Piers    []geom.Vec
}

// Plan is the bridge from the head from to the head to with its deck at deck, or a Refusal
// saying why none may stand there.
func Plan(g Ground, r Rules, from, to cell.ID, deck float32) (Bridge, error) {
	gr := g.Grid()
	b := Bridge{From: from, To: to, Deck: deck}
	way, spans, err := Line(gr, from, to, r.Longest)
	if err != nil {
		return Bridge{}, err
	}
	b.Way, b.Spans = way, spans
	for _, h := range [2]cell.ID{from, to} {
		if !g.Takes(h) {
			return Bridge{}, refusal(gr, h, "a head stands on ground its road may not take")
		}
		if _, ok := g.Bridged(h); ok {
			return Bridge{}, refusal(gr, h, "a head stands on another bridge")
		}
	}
	var buf []float32
	var at, ends []geom.Vec
	ends = gr.CellOutline(from, ends)
	ends = gr.CellOutline(to, ends)
	crosses := false
	for _, s := range spans {
		if p, ok := g.Bridged(s); ok && !p.Head {
			return Bridge{}, refusal(gr, s, "crosses another bridge")
		} else if ok {
			return Bridge{}, refusal(gr, s, "runs over another bridge's head")
		}
		for i := range gr.Directions() {
			n, ok := gr.Toward(s, i)
			if !ok || n == from || n == to || slices.Contains(spans, n) {
				continue
			}
			if p, ok := g.Bridged(n); ok && !p.Head {
				return Bridge{}, refusal(gr, s, "runs beside another bridge")
			}
		}
		buf = g.Heights(s, buf[:0])
		at = gr.CellOutline(s, at[:0])
		if len(buf) > 1 {
			for k := range buf {
				if on(at[k], ends, gr.CellSpan()) {
					buf[k] = deck // a head's, levelled
				}
			}
		}
		lo, hi := f32.Inf(1), middle(buf)
		for k, h := range buf {
			lo = min(lo, h)
			if len(buf) == 1 || !on(at[k], ends, gr.CellSpan()) {
				hi = max(hi, h)
			}
		}
		wet := g.Wet(s)
		if r.Heights {
			if hi > deck-r.Clearance {
				return Bridge{}, refusal(gr, s, "the ground rises to %.1f under a deck at %.1f", hi,
					deck)
			}
			if r.High > 0 && deck-lo > r.High {
				return Bridge{}, refusal(gr, s, "hangs %.1f over the ground, %.1f at most",
					deck-lo, r.High)
			}
			crosses = crosses || wet || lo <= deck-r.Under
		} else {
			crosses = crosses || wet || !g.Takes(s)
		}
		if err := under(g, r, s, way, deck, buf); err != nil {
			return Bridge{}, err
		}
	}
	if !crosses {
		if r.Heights {
			return Bridge{}, refusal(gr, from,
				"crosses no water and no valley deep enough to pass under")
		}
		return Bridge{}, refusal(gr, from, "crosses no water nor anything a road could not")
	}
	if r.Heights {
		for _, h := range [2]cell.ID{from, to} {
			if err := level(g, r, h, deck); err != nil {
				return Bridge{}, err
			}
		}
	}
	if r.Heights {
		b.Piers = Piers(gr, spans)
	}
	return b, nil
}

// Piers are where the piers of a bridge spanning spans stand, between every two of them.
func Piers(g grid.Grid, spans []cell.ID) []geom.Vec {
	var out []geom.Vec
	for i := 1; i < len(spans); i++ {
		a, c := g.CellCenter(spans[i-1]), g.CellCenter(spans[i])
		out = append(out, geom.NewVec((a.X+c.X)/2, (a.Y+c.Y)/2))
	}
	return out
}

// Line is the way a bridge from the head from runs to the head to and the cells it spans, or
// why it runs no way; longest cells between at most.
func Line(g grid.Grid, from, to cell.ID, longest int) (int, []cell.ID, error) {
	if from == to {
		return 0, nil, refusal(g, from, "begins and ends on one cell")
	}
	a, b := g.CellCenter(from), g.CellCenter(to)
	off := geom.NewVec(b.X-a.X, b.Y-a.Y)
	way, best := 0, f32.Inf(-1)
	for i := range g.Directions() {
		st := g.Step(i)
		if cos := (off.X*st.X + off.Y*st.Y) / f32.Hypot(st.X, st.Y); cos > best {
			way, best = i, cos
		}
	}
	var between []cell.ID
	at := from
	for range 4 * (longest + 2) {
		n, ok := g.Toward(at, way)
		if !ok {
			break
		}
		if n == to {
			switch {
			case len(between) == 0:
				return 0, nil, refusal(g, from, "spans no cell: its heads lie side by side")
			case len(between) > longest:
				return 0, nil, refusal(g, from, "spans %d cells, %d at most", len(between),
					longest)
			}
			return way, between, nil
		}
		between = append(between, n)
		at = n
	}
	return 0, nil, refusal(g, to, "lies off every line from the other head")
}

func level(g Ground, r Rules, h cell.ID, deck float32) error {
	var buf [4]float32
	heights := g.Heights(h, buf[:0])
	for _, v := range heights {
		if d := f32.Abs(v - deck); d > r.Reshape+1e-6*max(1, f32.Abs(deck)) {
			return refusal(g.Grid(), h, "a head's ground lies %.1f off the deck, %.1f at most", d,
				r.Reshape)
		}
	}
	return roads(g, h, deck, heights)
}

// roads refuses levelling h where a road beside it would lie as Transport Tycoon would not lay it.
func roads(g Ground, h cell.ID, deck float32, heights []float32) error {
	gr := g.Grid()
	if len(heights) == 1 {
		for i := range gr.Directions() {
			n, ok := gr.Toward(h, i)
			if !ok || !g.Road(n).Has(gr.Opposite(i)) {
				continue
			}
			if !ground.RoadJoins(g.Heights(n, nil)[0], deck) {
				return refusal(gr, n, "the road would not join the head levelled")
			}
		}
		return nil
	}
	moved := gr.CellOutline(h, nil)
	run := gr.CellSpan()
	var outline []geom.Vec
	for i := range gr.Directions() {
		n, ok := gr.Toward(h, i)
		links := g.Road(n)
		if !ok || links == 0 {
			continue
		}
		before := g.Heights(n, nil)
		if !ground.RoadFits(gr, n, before, links, run) {
			continue
		}
		after := slices.Clone(before)
		outline = gr.CellOutline(n, outline[:0])
		for k, p := range outline {
			if on(p, moved, run) {
				after[k] = deck
			}
		}
		if !ground.RoadFits(gr, n, after, links, run) {
			return refusal(gr, n, "levelling the head would break the road")
		}
	}
	return nil
}

func under(g Ground, r Rules, s cell.ID, way int, deck float32, heights []float32) error {
	gr := g.Grid()
	links := g.Road(s)
	if links == 0 {
		return nil
	}
	if !r.Heights {
		return refusal(gr, s, "runs over a road")
	}
	across := false
	for i := range gr.Directions() {
		if links == cell.Links(1<<i|1<<gr.Opposite(i)) && square(gr, i, way) {
			across = true
		}
	}
	if !across {
		return refusal(gr, s, "runs over a road not crossing it square")
	}
	for _, h := range heights {
		if h > deck-r.Under {
			return refusal(gr, s, "runs over a road too low to pass under")
		}
	}
	return nil
}

// a square cell's top is folded along one of its diagonals, its middle halfway along it
func middle(heights []float32) float32 {
	if len(heights) != 4 {
		return heights[0]
	}
	return max((heights[0]+heights[2])/2, (heights[1]+heights[3])/2)
}

func on(p geom.Vec, ends []geom.Vec, run float32) bool {
	return slices.ContainsFunc(ends, func(q geom.Vec) bool {
		return f32.Abs(p.X-q.X) < max(1e-6*run, f32.Eps(q.X)) &&
			f32.Abs(p.Y-q.Y) < max(1e-6*run, f32.Eps(q.Y))
	})
}

func square(g grid.Grid, a, b int) bool {
	sa, sb := g.Step(a), g.Step(b)
	dot := sa.X*sb.X + sa.Y*sb.Y
	return f32.Abs(dot) <= f32.Eps(1)*f32.Hypot(sa.X, sa.Y)*f32.Hypot(sb.X, sb.Y)
}

// Refusal is why a bridge may not stand where it was planned, of the cell At.
type Refusal struct {
	At   cell.ID
	why  string
	args []any
	g    grid.Grid
}

// Why is the rule broken.
func (r *Refusal) Why() string { return fmt.Sprintf(r.why, r.args...) }

func (r *Refusal) Error() string {
	x, y, _ := r.g.Coords(r.At)
	return fmt.Sprintf("the bridge at [%d, %d] %s", x, y, r.Why())
}

func refusal(g grid.Grid, c cell.ID, why string, args ...any) *Refusal {
	return &Refusal{At: c, why: why, args: args, g: g}
}
