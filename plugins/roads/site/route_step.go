package site

import (
	"math"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/hpastar/graph"
)

// forward is every step a road may take from from but back to prev's cell, every leap, and the
// end past from where from is the goal; a graded road runs straight on through a cell it cannot
// level, as a bend lies level
func (p *Planner) forward(from, prev routeNode,
	buf []graph.Transition[routeNode]) []graph.Transition[routeNode] {
	buf = buf[:0]
	if from.in == routeEnd {
		return buf
	}
	if from.c == p.goal {
		buf = append(buf, graph.Transition[routeNode]{To: routeNode{c: from.c, in: routeEnd}})
	}
	a := p.at(from.c)
	stiff := p.graded && !a.levels
	for i := range p.runs {
		if stiff && !p.onward(a, from.in, i) {
			continue
		}
		if b, ok := p.grid.Toward(from.c, i); ok && b != prev.c && b != from.c {
			rb := p.at(b)
			if cost, ok := p.step(from.c, a, rb, i); ok {
				if next, ok := p.into(b, rb, i); ok {
					buf = append(buf, graph.Transition[routeNode]{To: next, Cost: cost})
				}
			}
		}
		// a bridge's heads lie level
		if p.leaps == nil || stiff {
			continue
		}
		p.jumps = p.leaps(from.c, i, p.jumps[:0])
		for _, l := range p.jumps {
			if l.To == prev.c || l.To == from.c {
				continue
			}
			rh := p.at(l.To)
			if p.graded && !rh.levels {
				continue
			}
			// over the spans at the deck's price, onto the head at its ground's
			if price, ok := p.enter(rh); ok {
				cost := p.runs[i]*p.metre*(float64(l.Spans)+price) + float64(l.Cost)
				next := routeNode{c: l.To, in: routeAny}
				buf = append(buf, graph.Transition[routeNode]{To: next, Cost: cost})
			}
		}
	}
	return buf
}

// onward says whether a graded road may leave a cell it cannot level, come into by way in
// (routeAny at the start), by way out: straight on, along any road standing there
func (p *Planner) onward(a *routeCell, in int8, out int) bool {
	if in >= 0 {
		return out == int(in)
	}
	return a.road&^(1<<out|1<<p.back[out]) == 0
}

// into is the state a road stepping into b by way i comes to: any way on where it may level b,
// else straight on, which a road standing across b's axis forbids
func (p *Planner) into(b cell.ID, rb *routeCell, i int) (routeNode, bool) {
	if !p.graded || rb.levels {
		return routeNode{c: b, in: routeAny}, true
	}
	return routeNode{c: b, in: int8(i)}, rb.road&^(1<<i|1<<p.back[i]) == 0
}

// step is what the step from a, the cell from, onto its neighbour b, direction i, costs a road;
// false where it may not take it
func (p *Planner) step(from cell.ID, a, b *routeCell, i int) (float64, bool) {
	price, ok := p.enter(b)
	if !ok || !p.decks(a, b, i) {
		return 0, false
	}
	run := p.runs[i]
	if a.span() || b.span() || p.along(a, b, i) {
		// a span's height is not its ground's, and a road standing lies as it was laid
		return run * p.metre * price, true
	}
	if p.corner[i] && (!p.passes(from, i) || p.graded && !p.level(a, b)) {
		return 0, false
	}
	rise, ok := p.rise(a, b, i)
	if !ok {
		return 0, false
	}
	if b.road != 0 {
		rise = 0
	}
	return run * p.metre * price * (1 + p.climb*rise/run), true
}

// enter is what a metre of road over rc costs by its ground, 1 on a deck or a road standing;
// false where a road may not run onto it. A ford is ground, wet or not, where the rules allow it.
func (p *Planner) enter(rc *routeCell) (float64, bool) {
	switch {
	case rc.decked:
		return 1, p.r.Decks && (rc.road == 0 || p.r.Crossings)
	case rc.road != 0:
		return 1, p.r.Crossings
	case rc.ford:
		return float64(rc.price), p.r.Fords
	case rc.wet:
		return 0, false
	}
	return float64(rc.price), rc.takes
}

// decks says whether the step from a to b, direction i, keeps to the decks it meets: from a deck
// onto the next only where the two run to each other, onto and off a bridge's head on its ground
// side, and never onto or off a span across it
func (p *Planner) decks(a, b *routeCell, i int) bool {
	toward, back := a.deck.Has(i), b.deck.Has(p.back[i])
	if toward || back {
		return (toward || !a.decked) && (back || !b.decked)
	}
	return !a.span() && !b.span()
}

// along says whether the step from a to b, direction i, runs along a road standing
func (p *Planner) along(a, b *routeCell, i int) bool {
	return a.road.Has(i) && b.road.Has(p.back[i])
}

// passes says whether a road may cut the corner direction i leaves from by: the cells either side
// dry and taking or carrying a road, and no road standing through the corner, which a crossing
// there would meet in no cell
func (p *Planner) passes(from cell.ID, i int) bool {
	f1, f2, _ := p.grid.Flanks(i)
	c1, ok1 := p.grid.Toward(from, f1)
	c2, ok2 := p.grid.Toward(from, f2)
	if !ok1 || !ok2 {
		return false
	}
	s1, s2 := p.at(c1), p.at(c2)
	j := p.across[i]
	return s1.beside() && s2.beside() &&
		(j < 0 || !s1.road.Has(j) && !s2.road.Has(p.back[j]))
}

// a ford is water a road cuts no corner of, as any
func (rc *routeCell) beside() bool {
	return !rc.wet && !rc.ford && (rc.takes || rc.decked || rc.road != 0)
}

// level says whether a and b may both lie flat at one height, as a road through the corner they
// share lies (ground.RoadFits)
func (p *Planner) level(a, b *routeCell) bool {
	return float64(max(a.lo, b.lo)) <= float64(min(a.hi, b.hi))+p.eps
}

// rise is how far the step from a to b, direction i, climbs or falls between their middles once
// the levelling has dug the higher cell and filled the lower where their corners may move; false
// where steeper than a road may run
func (p *Planner) rise(a, b *routeCell, i int) (float64, bool) {
	ha, hb := float64(a.h), float64(b.h)
	rise := math.Abs(hb - ha)
	if p.levelling {
		high, low := b, a
		if hb < ha {
			high, low = a, b
		}
		if !high.fixed {
			rise -= float64(p.cut)
		}
		if !low.fixed {
			rise -= float64(p.fill)
		}
		rise = max(rise, 0)
	}
	return rise, rise <= p.steep*p.runs[i]+p.eps
}
