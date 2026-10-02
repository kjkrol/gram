package plugin

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/uid"
)

// Rule is what is done at a moment a Plugin catches in its own pass over its entities, built with
// rule.On (or plugin/host); its moment's type says which plugin hosts it, and another refuses it.
type Rule any

// ErrUnhosted is what Hook reports for a rule the Plugin cannot run.
var ErrUnhosted = errors.New("plugin: rule cannot be hosted here")

// ErrHostBuilt is what hooking a rule reports once its host's queries exist.
var ErrHostBuilt = errors.New("plugin: rule hooked after its host was built")

// Tick is what a hosted rule is told about the pass it runs in.
type Tick struct {
	CmdBuf   *goke.CmdBuf     // structural changes land when the pass is over
	Now      time.Time        // read once for the whole pass
	Dt       time.Duration    // length of this tick
	Commands *control.Carrier // the world's, for the commands an entity gives itself
	Time     time.Duration    // the game time the step ends at, on the world's clock
	Seed     uint64           // the world's seed, which a rule's Chance draws from
	World    uid.UID64        // the world's own entity, the clock's: the game's states, for During
	// Around tells each the places — entities of their own, a board's cells — within rings of where
	// the moment stands, those it stands on first: its host's to say, for a rule's Here and Around;
	// nil where there are none.
	Around func(moment any, rings int, each func(uid.UID64))
}

// TickSource builds the Tick of a pass: the world's (world.Plugin.Tick), which a host is handed.
type TickSource func(cb *goke.CmdBuf, dt time.Duration) Tick

// Of is the Tick of a pass over dt; with no source, one of no world — no carrier, no game time.
func (f TickSource) Of(cb *goke.CmdBuf, dt time.Duration) Tick {
	if f == nil {
		return Tick{CmdBuf: cb, Now: time.Now(), Dt: dt}
	}
	return f(cb, dt)
}

// MaxFamilies is how many tag families one host's rules may name between them.
const MaxFamilies = 8

// Marks is which tags of a host's families one entity carries — what a host reads from an
// entity and hands back to Dispatch, and what a payload passes on for Carries.
type Marks struct {
	words    [MaxFamilies]uint64
	families *[]reflect.Type
}

// MarksOf is what a host builds from the words it read, one per family in families' order.
func MarksOf(words [MaxFamilies]uint64, families *[]reflect.Type) Marks {
	return Marks{words: words, families: families}
}

// Word is the bits read for the host's family at index i.
func (m Marks) Word(i int) uint64 { return m.words[i] }

// Carries reports whether the entity behind m carries t; the host must name t's family in a
// rule, or it never read it.
func (m Marks) Carries[F any](t tag.Tag[F]) bool {
	if m.families == nil {
		return false
	}
	want := reflect.TypeFor[F]()
	for i, known := range *m.families {
		if known == want {
			return m.words[i]&(1<<t) != 0
		}
	}
	panic(fmt.Sprintf("plugin: Carries asked about family %v, which no rule of this host names", want))
}
