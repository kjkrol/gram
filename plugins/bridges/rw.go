package bridges

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/bridges/site"
)

// Write changes the bridges standing, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the bridges standing, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Spawn builds every bridge a level's file has, a line `{kind: <a kind of bridge>, from: [x, y],
// to: [x, y]}`, its deck at its first head's level. One the Rules refuse fails Populate with its
// line.
func (w *Write) Spawn(level *board.Level) error {
	p := w.p
	heights := level.Heights()
	g := newGround(p.board.Read(0).Board(), heights, 0)
	gr := g.Grid()
	return board.Each(level, func(name string, r Row, at string) error {
		t := p.types.Named(name)
		way, spans, err := site.Line(gr, r.From, r.To, p.cfg.Rules.Longest)
		if err != nil {
			return err
		}
		b := site.Bridge{From: r.From, To: r.To, Way: way, Deck: g.level(r.From), Spans: spans}
		if level.Heights() != nil {
			b.Piers = site.Piers(gr, spans)
		}
		p.laid = append(p.laid, laid{t: t, plan: b, at: at, heights: heights})
		p.worldPlugin.Write().Seed(p.parts(t, b, g)...)
		return nil
	})
}
