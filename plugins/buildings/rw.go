package buildings

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
)

// Write changes the buildings standing, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the buildings standing, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Spawn stands every building a level's file has, a line `{kind: <a kind of building>,
// mid: [x, y], size: [length, width], turn: degrees, floors: n, door: [x, y]}`: drawn from then
// on, an entity where its settlement's sector unfolds; one with a door stands its room too.
func (w *Write) Spawn(level *board.Level) error {
	return board.Each(level, func(name string, row Row, _ string) error {
		return w.stand(name, row, level.Heights())
	})
}

// Stand stands a building of the kind named name as row says, as a line of a level's file does;
// one with a door, in the Spawn section, on the ground its level's file seeds (Spawn) or the
// board's heights.
func (w *Write) Stand(name string, row Row) {
	var ground func(geom.Vec) float32
	if heights := w.p.board.Read(0).Heights(); heights != nil {
		ground = heights.At
	}
	if err := w.stand(name, row, ground); err != nil {
		panic(fmt.Sprintf("buildings: a %s at %v: %v", name, row.Mid, err))
	}
}

func (w *Write) stand(name string, row Row, ground func(geom.Vec) float32) error {
	p := w.p
	spec := p.types.Named(name)
	if row.Door != [2]float32{} {
		if ground == nil {
			return fmt.Errorf("a building with a door stands on the ground's heights, none yet")
		}
		h, err := p.houseOf(spec, row, ground)
		if err != nil {
			return err
		}
		p.lay(name, h, ground)
	}
	b := spec.of(row)
	k := p.worldPlugin.Read(0).SectorOf(b.mid.X, b.mid.Y)
	p.Res.Stood[k] = append(p.Res.Stood[k], Stood{Kind: name, Row: row})
	p.solids.stale, p.cage.stale = true, true
	return nil
}

// OpenRooms has a house a unit stands in open round its room before a camera of the tactical
// view, loose or centred on a unit, or stand whole there (Settings.Shut).
func (w *Write) OpenRooms(on bool) { w.p.Res.Settings.Shut = !on }

// OpenRoomsBehind has a house a unit stands in open round its room before a camera behind a unit,
// or stand whole there (Settings.ShutBehind).
func (w *Write) OpenRoomsBehind(on bool) { w.p.Res.Settings.ShutBehind = !on }
