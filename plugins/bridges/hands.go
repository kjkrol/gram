package bridges

import (
	"fmt"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/bridges/site"
	"github.com/kjkrol/uid"
)

// hand is the bridge a player lays out: its first head once marked, the head the cursor points
// the bridge on to, the bridge planned between them or why none may stand there, and whether the
// player is asked to build it.
type hand struct {
	t      Type
	from   cell.ID
	marked bool
	to     cell.ID
	aimed  bool
	plan   site.Bridge
	err    error
	asked  bool
	seen   uint64 // the tick it was last aimed in
}

// pending is a bridge its player said to build, built at the next step of the simulation.
type pending struct {
	t    Type
	plan site.Bridge
}

// forget is how many ticks a hand nobody aims is kept: the cursor off the picture a moment.
const forget = 30

// carryOut carries out the players' commands about the bridges in their hands, at once.
func (p *Plugin) carryOut() {
	p.tick++
	p.aims.Drain(func(i control.Issued[Aim]) {
		h := p.handOf(i.Player)
		h.seen = p.tick
		if !h.marked || h.asked {
			return
		}
		if to, ok := p.endAt(h.from, i.Command.At); !ok {
			h.aimed, h.err = false, nil
		} else if !h.aimed || to != h.to {
			h.to, h.aimed = to, true
			p.plan(h)
		}
	})
	p.marks.Drain(func(i control.Issued[Mark]) {
		h := p.handOf(i.Player)
		h.seen = p.tick
		c, ok := p.board.Board().CellAt(i.Command.At)
		switch {
		case !ok || h.asked:
		case !h.marked:
			h.from, h.marked, h.aimed, h.err = c, true, false, nil
		case c == h.from:
			h.marked, h.aimed, h.err = false, false, nil
		case h.aimed && h.err == nil:
			h.asked = true
		}
	})
	p.builds.Drain(func(i control.Issued[Build]) {
		if h := p.hands[i.Player]; h != nil && h.asked {
			p.pending = append(p.pending, pending{t: h.t, plan: h.plan})
			h.marked, h.aimed, h.asked, h.err = false, false, false, nil
		}
	})
	p.cancels.Drain(func(i control.Issued[Cancel]) {
		if h := p.hands[i.Player]; h != nil {
			h.asked = false
		}
	})
	for id, h := range p.hands {
		if !h.asked && p.tick-h.seen > forget {
			delete(p.hands, id)
		}
	}
}

func (p *Plugin) handOf(id control.PlayerID) *hand {
	h := p.hands[id]
	if h == nil {
		h = &hand{t: p.building()}
		p.hands[id] = h
	}
	return h
}

// plan plans the bridge from the hand's first head to the head it points to, its deck at the
// first head's level.
func (p *Plugin) plan(h *hand) {
	g := p.ground(h.t)
	h.plan, h.err = site.Plan(g, p.rules(), h.from, h.to, g.level(h.from))
}

// endAt is the cell a bridge from the cell from runs to with the cursor at the world point at:
// along the grid's direction nearest the cursor, as many steps as the cursor lies along it.
func (p *Plugin) endAt(from cell.ID, at geom.Vec) (cell.ID, bool) {
	g := p.board.Board().Grid
	m := g.CellCenter(from)
	off := geom.NewVec(at.X-m.X, at.Y-m.Y)
	best, steps, along := -1, 0, -math.MaxFloat64
	for i := range g.Directions() {
		s := g.Step(i)
		length := math.Hypot(s.X, s.Y)
		cos := (off.X*s.X + off.Y*s.Y) / (length * max(math.Hypot(off.X, off.Y), 1e-9))
		if cos > along {
			best, along = i, cos
			steps = int(math.Round((off.X*s.X + off.Y*s.Y) / (length * length)))
		}
	}
	if best < 0 || steps < 1 {
		return 0, false
	}
	c := from
	for range steps {
		n, ok := g.Toward(c, best)
		if !ok {
			break
		}
		c = n
	}
	return c, c != from
}

// text is a window's or a label's words, said off the hands: say picks them.
type text struct {
	p   *Plugin
	say func(h *hand) (string, bool)
}

func (t text) Text(uid.UID64, bool) (string, bool) {
	for _, h := range t.p.hands {
		if s, ok := t.say(h); ok {
			return s, true
		}
	}
	return "", false
}

func askText(h *hand) (string, bool) { return "Build the bridge?", h.asked }

func spansText(h *hand) (string, bool) {
	if !h.asked {
		return "", false
	}
	n := len(h.plan.Spans)
	if n == 1 {
		return "It spans one cell.", true
	}
	return fmt.Sprintf("It spans %d cells, on %d piers.", n, len(h.plan.Piers)), true
}

func reasonText(h *hand) (string, bool) {
	if !h.marked || !h.aimed || h.asked || h.err == nil {
		return "", false
	}
	return h.err.Error(), true
}
