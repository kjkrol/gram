package forest

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Plant is the command an aggregate gives itself under a world.Unfold to have the trees the
// level's file stands in the stretch unfolding spawned as its parts.
type Plant struct{ in uid.UID64 }

// Aim tells the command the stretch of the aggregate unfolding.
func (c *Plant) Aim(who uid.UID64) { c.in = who }

func (Plant) Command() {}

var (
	_ command.Command = Plant{}
	_ host.Aimed      = (*Plant)(nil)
)

// Queues are the plugin's command queues: Plant.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.plants}
}

// Write changes the trees standing by themselves, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the trees standing by themselves, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Spawn takes every tree a level's file stands by itself, a line `{kind: <a kind of tree>,
// at: [x, y]}`: an entity where the aggregate over its sector unfolds and gives itself Plant.
func (w *Write) Spawn(level *board.Level) error {
	return board.Each(level, func(name string, row Row, _ string) error {
		w.Stand(name, row)
		return nil
	})
}

// Stand stands a tree of the kind named name by itself as row says, as a line of a level's
// file does.
func (w *Write) Stand(name string, row Row) {
	p := w.p
	k := p.worldPlugin.Read(0).SectorOf(row.At[0], row.At[1])
	at := geom.NewVec(row.At[0], row.At[1])
	p.Res.Planted[k] = append(p.Res.Planted[k], Planted{Kind: name, At: at})
}

// plant gives the aggregate of each Plant the trees standing in the stretch it was aimed at; a
// preview for the sheets gets none
func (p *Plugin) plant() {
	r := p.worldPlugin.Read(0)
	carrier := p.worldPlugin.Write().Carrier()
	p.plants.Drain(func(i command.Issued[Plant]) {
		st, ok := r.Unfolding(i.Command.in)
		if !i.ByEntity || !ok || st.Preview || st.Whole != i.Entity {
			return
		}
		mid := geom.NewVec((st.Box.TopLeft.X+st.Box.BottomRight.X)/2,
			(st.Box.TopLeft.Y+st.Box.BottomRight.Y)/2)
		for _, t := range p.Res.Planted[st.Sector] {
			carrier.PutFrom(i.Entity, world.SpawnPartIn(i.Command.in,
				kind.EntryOf(t.Kind, Row{}), t.At.Sub(mid)))
		}
	})
}

// DefaultBindings are none: Plant is an aggregate's own.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }
