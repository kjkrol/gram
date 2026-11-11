package main

import (
	"errors"
	"slices"

	"github.com/kjkrol/gram/plugins/board/cell"
)

// how many legs of a guide are levelled together, how far off its trace a leg may run, how
// many times dearer a cell off it is, and how often a leg the levelling refused is planned anew
const (
	batch    = 16
	corridor = 2
	astray   = 2
	retries  = 2
	// the fewest cells of a leg's trace a half of it is tried over
	shortest = 4
)

// a leg planned and marked, not yet levelled with the roads laid
type pending struct {
	leg leg
	rd  road
}

// follow lays the guide's legs, each planned along its trace, a batch levelled together with
// every road laid; a leg that cannot be laid is left out and named
func (r *roads) follow(g *guide) {
	r.spannable = g.spannable
	defer func() { r.spannable, r.corridor, r.traced = nil, nil, nil }()
	r.moved = map[cell.ID]cell.ID{}
	var waiting []pending
	r.legs = len(g.legs)
	for _, l := range g.legs {
		var ok bool
		if l.from, ok = r.stand(l.from, r.moved); !ok {
			continue
		}
		if l.to, ok = r.stand(l.to, r.moved); !ok || l.from == l.to {
			continue
		}
		rd, err := r.along(l)
		if err != nil {
			r.split(l, err)
			continue
		}
		r.mark(rd)
		r.waiting = append(r.waiting, rd)
		waiting = append(waiting, pending{l, rd})
		if len(waiting) == batch {
			r.commit(waiting)
			waiting = waiting[:0]
		}
	}
	r.commit(waiting)
}

// stand is the cell a leg ends on for the junction at c: c, or the nearest a road may take
// within the corridor, the same for every leg ending there
func (r *roads) stand(c cell.ID, moved map[cell.ID]cell.ID) (cell.ID, bool) {
	if m, ok := moved[c]; ok {
		return m, m != c || (chartGround{r: r}).Takes(c)
	}
	moved[c] = c
	if (chartGround{r: r}).Takes(c) {
		return c, true
	}
	cx, cy := r.w.xy(c)
	best, found := 0, false
	for y := max(cy-corridor, 0); y <= min(cy+corridor, r.w.rows-1); y++ {
		for x := max(cx-corridor, 0); x <= min(cx+corridor, r.w.cols-1); x++ {
			d := (x-cx)*(x-cx) + (y-cy)*(y-cy)
			if n := r.w.cell(x, y); (chartGround{r: r}).Takes(n) && (!found || d < best) {
				moved[c], best, found = n, d, true
			}
		}
	}
	return moved[c], found
}

// along plans l's road within the corridor round its trace, planned anew with a cell barred or
// a bridge banned while it crosses itself or a bridge as no road may
func (r *roads) along(l leg) (road, error) {
	r.lead(l)
	clear(r.barred)
	clear(r.banned)
	clear(r.dear)
	clear(r.offered)
	return r.routed(l)
}

func (r *roads) routed(l leg) (road, error) {
	p := r.planners[len(r.planners)-1]
	for range tries {
		cells, err := p.Route(l.from, l.to, r.route[:0])
		if err != nil {
			return road{}, err
		}
		r.route = cells
		rd := r.unfold(cells)
		if r.clash(rd, l.from, l.to) {
			clear(r.offered)
			continue
		}
		return rd, nil
	}
	return road{}, errors.New("no way but across itself or a bridge")
}

// lead has the planner keep to the corridor round l's trace
func (r *roads) lead(l leg) {
	if r.corridor == nil {
		r.corridor, r.traced = map[cell.ID]bool{}, map[cell.ID]bool{}
	}
	clear(r.corridor)
	clear(r.traced)
	for _, c := range append([]cell.ID{l.from, l.to}, l.trace...) {
		r.traced[c] = true
		cx, cy := r.w.xy(c)
		for y := max(cy-corridor, 0); y <= min(cy+corridor, r.w.rows-1); y++ {
			for x := max(cx-corridor, 0); x <= min(cx+corridor, r.w.cols-1); x++ {
				r.corridor[r.w.cell(x, y)] = true
			}
		}
	}
}

// commit levels the roads waiting with every road laid and lays them; where the levelling
// refuses, the leg through the cell it names is taken out and tried alone, the rest again
func (r *roads) commit(waiting []pending) {
	if len(waiting) == 0 {
		return
	}
	rds := slices.Clone(r.laid)
	var flat []height
	for _, p := range waiting {
		rds = append(rds, p.rd)
		flat = append(flat, r.flatten(p.rd)...)
	}
	l := r.layout(rds)
	est, err := r.levels(&l)
	if err == nil {
		r.laid, r.graded = rds, est
		for _, p := range waiting {
			r.waiting = slices.DeleteFunc(r.waiting, func(rd road) bool {
				return &rd.cells[0] == &p.rd.cells[0]
			})
		}
		return
	}
	r.undo(flat)
	if len(waiting) == 1 {
		r.alone(waiting[0], err)
		return
	}
	if at, ok := refusedAt(err); ok {
		i := slices.IndexFunc(waiting, func(p pending) bool {
			return slices.Contains(p.rd.cells, at)
		})
		if i >= 0 {
			one := waiting[i]
			r.commit(slices.Delete(slices.Clone(waiting), i, i+1))
			r.commit([]pending{one})
			return
		}
	}
	half := len(waiting) / 2
	r.commit(slices.Clone(waiting[:half]))
	r.commit(slices.Clone(waiting[half:]))
}

// alone takes the leg of p, which the levelling refused for why, out of those waiting and lays
// what of it can be laid by itself
func (r *roads) alone(p pending, why error) {
	r.waiting = slices.DeleteFunc(r.waiting, func(rd road) bool {
		return &rd.cells[0] == &p.rd.cells[0]
	})
	r.remark()
	r.retry(p.leg, p.rd, why)
}

// retry plans l, whose road rd the levelling refused for why, anew round what it refused and
// lays it once it can be levelled; else each half of it is tried
func (r *roads) retry(l leg, rd road, why error) {
	r.lead(l)
	clear(r.barred)
	clear(r.banned)
	clear(r.dear)
	clear(r.offered)
	for range retries {
		if !r.away(rd, why, l.from, l.to) {
			break
		}
		var err error
		if rd, err = r.routed(l); err != nil {
			why = err
			break
		}
		if why = r.trial(rd); why == nil {
			r.laid = append(r.laid, rd)
			return
		}
	}
	r.split(l, why)
}

// split lays the halves of l, which cannot be laid whole for why, each by itself, and theirs
// in turn: what of a long road can be laid is, the stretch that cannot left out and named
func (r *roads) split(l leg, why error) {
	if len(l.trace) < 2*shortest {
		r.skip(l, why)
		return
	}
	m := len(l.trace) / 2
	mid, ok := r.stand(l.trace[m], r.moved)
	if !ok {
		r.skip(l, why)
		return
	}
	for _, half := range [2]leg{
		{from: l.from, to: mid, trace: l.trace[:m+1], class: l.class},
		{from: mid, to: l.to, trace: l.trace[m:], class: l.class},
	} {
		if half.from == half.to {
			continue
		}
		rd, err := r.along(half)
		if err != nil {
			r.split(half, err)
			continue
		}
		if err := r.trial(rd); err != nil {
			r.retry(half, rd, err)
			continue
		}
		r.laid = append(r.laid, rd)
	}
}

// remark marks the roads laid and those waiting anew, one taken out
func (r *roads) remark() {
	clear(r.marks)
	for _, rd := range r.laid {
		r.mark(rd)
	}
	for _, rd := range r.waiting {
		r.mark(rd)
	}
}

func (r *roads) skip(l leg, why error) {
	r.lost = append(r.lost, l)
	r.skipped = append(r.skipped, skip{r.w.point(l.from), r.w.point(l.to), why})
}
