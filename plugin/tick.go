package plugin

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Tick is what a rule-driven system tells the rules it runs about its pass.
type Tick struct {
	CmdBuf   *goke.CmdBuf     // structural changes land when the pass is over
	Now      time.Time        // read once for the whole pass
	Dt       time.Duration    // length of this tick
	Commands *control.Carrier // the world's, for the commands an entity gives itself
	Effects  *effect.Effects  // the world's, which a rule's Apply, Keep and Dispel cast
	Time     time.Duration    // the game time the step ends at, on the world's clock
	Seed     uint64           // the world's seed, which a rule's Chance draws from
	World    uid.UID64        // the world's own entity, the clock's: the game's states, for During
	// Around tells each the places — entities of their own, a board's cells — within rings of where
	// the moment stands, those it stands on first: the system's to say, for a rule's Here and
	// Around; nil where there are none.
	Around func(moment any, rings int, each func(uid.UID64))
	// Wires tells the wire an entity is wired to (rule.Wired), for a rule's OnWire and WhileWire:
	// the world's; nil where there are none.
	Wires func(id uid.UID64) (uid.UID64, bool)
}

// TickSource builds the Tick of a pass: the world's (world.Plugin.Tick).
type TickSource func(cb *goke.CmdBuf, dt time.Duration) Tick

// Of is the Tick of a pass over dt; with no source, one of no world — no carrier, no game time.
func (f TickSource) Of(cb *goke.CmdBuf, dt time.Duration) Tick {
	if f == nil {
		return Tick{CmdBuf: cb, Now: time.Now(), Dt: dt}
	}
	return f(cb, dt)
}

// maxFamilies is how many tag families the pair rules of one system may name between them.
const maxFamilies = 8

// Marks is which tags of a system's families one entity carries: what PairRules reads off an
// entity and hands back to Dispatch.
type Marks struct{ words [maxFamilies]uint64 }
