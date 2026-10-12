package plugin

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Tick is what a host tells the rules it runs about its pass.
type Tick struct {
	CmdBuf   *goke.CmdBuf
	Now      time.Time // read once for the whole pass
	Dt       time.Duration
	Commands *control.Carrier
	Effects  *effect.Effects
	Time     time.Duration // game time at the step's end
	Seed     uint64
	World    uid.UID64 // the world's own entity
	// Around tells each the places within rings of where the moment stands, those it stands on
	// first; nil where there are none.
	Around func(moment any, rings int, each func(uid.UID64))
	// Roles tells the roles an entity plays, a bit each; nil where there are none.
	Roles func(id uid.UID64) uint64
}

// TickSource builds the Tick of a pass.
type TickSource func(cb *goke.CmdBuf, dt time.Duration) Tick

// Of is the Tick of a pass over dt; with no source, one of no world.
func (f TickSource) Of(cb *goke.CmdBuf, dt time.Duration) Tick {
	if f == nil {
		return Tick{CmdBuf: cb, Now: time.Now(), Dt: dt}
	}
	return f(cb, dt)
}

const maxFamilies = 8

// Marks is which tags of a host's families one entity carries, as PairRules reads them.
type Marks struct{ words [maxFamilies]uint64 }
