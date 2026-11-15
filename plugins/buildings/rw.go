package buildings

import "github.com/kjkrol/gram/plugins/board"

// Write changes the buildings standing, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the buildings standing, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Spawn stands every building a level's file has, a line `{kind: <a kind of building>,
// mid: [x, y], size: [length, width], turn: degrees, floors: n}`: drawn from then on, an entity
// where its settlement's sector unfolds.
func (w *Write) Spawn(level *board.Level) error {
	return board.Each(level, func(name string, row Row, _ string) error {
		w.Stand(name, row)
		return nil
	})
}

// Stand stands a building of the kind named name as row says, as a line of a level's file does.
func (w *Write) Stand(name string, row Row) {
	p := w.p
	b := p.types.Named(name).of(row)
	k := p.worldPlugin.Read(0).SectorOf(b.mid.X, b.mid.Y)
	p.Res.Stood[k] = append(p.Res.Stood[k], Stood{Kind: name, Row: row})
	p.solids.stale = true
}
