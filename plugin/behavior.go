package plugin

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
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
