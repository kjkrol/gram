package underground

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/underground/site"
)

// Write changes what lies under the ground, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes what lies under the ground, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Spawn digs every structure a level's file has: a tunnel, a line `{kind: <a kind of tunnel>,
// from: [x, y], to: [x, y], via: [[x, y], ...]}`, and a chamber, a line `{kind: <a kind of
// chamber>, from: [x, y], to: [x, y], depth: <units>}`. One the Rules refuse fails with its
// line, here or in Populate; the chambers are dug first, so a tunnel opens into one wherever the
// file has it.
func (w *Write) Spawn(level *board.Level) error {
	p := w.p
	heights := level.Heights()
	g := newGround(p.board.Read(0).Board(), heights, 0)
	// the board's cells are laid after: what a head's ground takes is Populate's to hold
	g.unlaid = true
	rules := p.cfg.Rules
	rules.Reshape = 1e-3 * g.Grid().CellSpan()
	err := board.Each(level, func(name string, r RoomRow, at string) error {
		t := p.types.Named(name)
		g.allows = t.spec.Allows
		ch, err := site.Room(g, rules, t.spec.Clear, r.Depth, r.From, r.To)
		if err != nil {
			return err
		}
		l := laid{t: t, chamber: ch, depth: r.Depth, at: at, heights: heights}
		p.laid = append(p.laid, l)
		whole, pieces := p.chamber(t, ch, g)
		p.worldPlugin.Write().SeedWhole(whole, pieces...)
		l.note(g)
		return nil
	})
	if err != nil {
		return err
	}
	return board.Each(level, func(name string, r Row, at string) error {
		t := p.types.Named(name)
		g.allows = t.spec.Allows
		tn, err := site.Plan(g, rules, t.spec.Clear, r.From, r.To, r.Via...)
		if err != nil {
			return err
		}
		l := laid{t: t, tunnel: tn, via: r.Via, at: at, heights: heights}
		p.laid = append(p.laid, l)
		whole, pieces := p.tunnel(t, tn, g)
		p.worldPlugin.Write().SeedWhole(whole, pieces...)
		l.note(g)
		return nil
	})
}
