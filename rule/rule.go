package rule

import (
	"errors"
	"fmt"

	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/rule/internal/engine"
	"github.com/kjkrol/uid"
)

// Rule is what is done at a moment a plugin catches in its own pass over its entities — a unit
// standing on the board, one seeing another, two striking — built with On and hooked with the
// plugin's Hook. The moment's type says which plugin hosts it; another refuses it with
// ErrUnhosted.
type Rule interface{ rule() }

// ErrUnhosted is what Hook reports for a rule the plugin cannot run.
var ErrUnhosted = errors.New("rule: rule cannot be hosted here")

// ErrHostBuilt is what hooking a rule reports once its host's queries exist.
var ErrHostBuilt = errors.New("rule: rule hooked after its host was built")

// On is a rule, named name: at every moment P a plugin's pass catches — a unit standing on the
// board, one seeing another, two striking — for whom filter lets through, it runs the steps body
// writes for the Moment, steps done within the pass alone. Hook it on the plugin that catches P:
// board.Plugin.Hook, vision's, collision's, world's, navigation's. A rule keeps no memory of its
// own: an effect's presence is its memory.
func On[P any](name string, filter Filter, body func(m *Moment[P]) Step) Rule {
	return build[P](name, filter, body(&Moment[P]{}))
}

// Filter is whom a rule fires for: All, Self, Between or Having.
type Filter struct {
	self, other pairSide
	paired      bool  // Self or Between
	others      bool  // Between
	mine        *side // Self, for a moment that is not Met
	need        *having
}

// All lets every entity the plugin shows the rule through — every pair, for a moment that is Met.
var All Filter

// Self lets through an entity carrying t.
func Self[F any](t tag.Tag[F]) Filter {
	return Filter{self: pairSideOf(t), other: anyPairSide, paired: true,
		mine: &side{tags: func() ruleState { return stateOf[tag.Tags[F]]() },
			carries: func(state any) bool { return state.(*tag.Tags[F]).Has(t) }}}
}

// Between lets through a pair whose entity carries a and whose other carries b — tag.Any for
// either — for a moment that is Met.
func Between[FA, FB any](a tag.Tag[FA], b tag.Tag[FB]) Filter {
	return Filter{self: pairSideOf(a), other: pairSideOf(b), paired: true, others: true}
}

// Having lets through an entity carrying the component T.
func Having[T any]() Filter {
	return Filter{need: &having{state: func() ruleState { return stateOf[T]() }}}
}

// side is a Self filter; on a moment that is not Met it reads the entity's tags as the rule's
// state.
type side struct {
	tags    func() ruleState
	carries func(state any) bool
}

// having is a Having filter.
type having struct{ state func() ruleState }

// fired is a rule's body, run on each moment its plugin hands it.
type fired[P any] struct {
	run     *engine.Instant
	current P
	about   bool // P is About an entity
	subject bool // P names a Subject, whom an Aimed command is about
}

func (f *fired[P]) fire(t Tick, _ any, about P) {
	f.current = about
	var id, subject uid.UID64
	var aimed bool
	if f.about {
		id = any(&f.current).(About).Who() // a pointer: no copy to the heap
	}
	if f.subject {
		subject, aimed = any(&f.current).(Subject).Subject()
	}
	p := engine.Pass{Commands: t.Commands, Dt: t.Dt, Time: t.Time, Seed: t.Seed, World: t.World, Around: t.Around}
	f.run.Fire(p, t.CmdBuf, id, f.about, subject, aimed)
}

// build is the rule named name over root, for whom filter lets through, as its plugin's host
// takes it.
func build[P any](name string, filter Filter, root Step) Rule {
	f := &fired[P]{}
	f.run = engine.NewInstant(name, root, &f.current)
	_, f.about = any(&f.current).(About)
	_, f.subject = any(&f.current).(Subject)
	_, met := any(*new(P)).(Met)
	switch {
	case filter.need != nil:
		return newEachWith[P](filter.need.state(), f.fire)
	case filter.paired && !met:
		if filter.mine == nil || filter.others {
			panic(fmt.Sprintf("rule: %q: Between needs a moment with others (Met)", name))
		}
		carries := filter.mine.carries
		return newEachWith[P](filter.mine.tags(), func(t Tick, state any, about P) {
			if carries(state) {
				f.fire(t, nil, about)
			}
		})
	case filter.paired || met:
		return pairOf[P](filter.self, filter.other, func(t Tick, about P) { f.fire(t, nil, about) })
	}
	return &every[P]{react: func(t Tick, about P) { f.fire(t, nil, about) }}
}
