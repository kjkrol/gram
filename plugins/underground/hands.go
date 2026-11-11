package underground

import (
	"fmt"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/underground/site"
	"github.com/kjkrol/uid"
)

type hand struct {
	t      Type
	from   cell.ID
	marked bool
	to     cell.ID
	aimed  bool
	plan   site.Tunnel
	err    error
	asked  bool
	seen   uint64
}

// forget is the ticks a hand nobody aims is kept: the cursor off the picture a moment.
const forget = 30

func (p *Plugin) carryOut() {
	p.tick++
	p.aims.Drain(func(i command.Issued[Aim]) {
		h := p.handOf(i.Issuer)
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
	p.marks.Drain(func(i command.Issued[Mark]) {
		h := p.handOf(i.Issuer)
		h.seen = p.tick
		c, ok := p.board.Read(0).Board().CellAt(i.Command.At)
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
	p.digs.Drain(func(i command.Issued[Dig]) {
		if h := p.hands[i.Issuer]; h != nil && h.asked {
			p.Res.Orders = append(p.Res.Orders, Order{Kind: h.t.Name(), Plan: h.plan})
			h.marked, h.aimed, h.asked, h.err = false, false, false, nil
		}
	})
	p.courses.Drain(func(i command.Issued[Courses]) {
		p.Res.Settings.HideCourses = !i.Command.Shown
	})
	p.cancels.Drain(func(i command.Issued[Cancel]) {
		if h := p.hands[i.Issuer]; h != nil {
			h.asked = false
		}
	})
	for id, h := range p.hands {
		if !h.asked && p.tick-h.seen > forget {
			delete(p.hands, id)
		}
	}
}

func (p *Plugin) handOf(id command.Issuer) *hand {
	h := p.hands[id]
	if h == nil {
		h = &hand{t: p.digging()}
		p.hands[id] = h
	}
	return h
}

func (p *Plugin) plan(h *hand) {
	g := p.ground(h.t)
	h.plan, h.err = site.Plan(g, p.rules(h.t), h.t.spec.Clear, h.from, h.to,
		site.Square(g.Grid(), h.from, h.to)...)
}

// endAt is the cell a tunnel from from runs to as the cursor points at at: along the way
// nearest it, as many cells as it lies off
func (p *Plugin) endAt(from cell.ID, at geom.Vec) (cell.ID, bool) {
	g := p.board.Read(0).Board().Grid
	m := g.CellCenter(from)
	off := geom.NewVec(at.X-m.X, at.Y-m.Y)
	best, steps, along := -1, 0, float32(-math.MaxFloat32)
	for i := range g.Directions() {
		s := g.Step(i)
		length := f32.Hypot(s.X, s.Y)
		cos := (off.X*s.X + off.Y*s.Y) / (length * max(f32.Hypot(off.X, off.Y), 1e-9))
		if cos > along {
			best, along = i, cos
			steps = int(f32.Round((off.X*s.X + off.Y*s.Y) / (length * length)))
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

func askText(h *hand) (string, bool) { return "Dig the tunnel?", h.asked }

func cellsText(h *hand) (string, bool) {
	if !h.asked {
		return "", false
	}
	if n := len(h.plan.Cells); n != 1 {
		return fmt.Sprintf("It runs under %d cells.", n), true
	}
	return "It runs under one cell.", true
}

func reasonText(h *hand) (string, bool) {
	if !h.marked || !h.aimed || h.asked || h.err == nil {
		return "", false
	}
	return h.err.Error(), true
}
