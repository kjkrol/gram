package host

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
)

// pair is a pair rule with its tags erased: what a host finds in Hook.
type pair[P any] struct {
	a, b  tagged
	same  bool
	react func(plugin.Tick, P)

	// fa and fb are the host's family indices of a and b; -1 for Any.
	fa, fb int
}

// PairHost runs the pair rules made for payload P inside a host's own pass. It reads every
// family its rules name off the entities it is shown and matches pairs by tag bits.
type PairHost[P any] struct {
	families []tagProbe
	kinds    []reflect.Type // the families' types, in the same order, for Marks
	pairs    []*pair[P]
	bound    bool
	matched  []int // DispatchGrouped's scratch
}

// Add takes a pair rule for P; ErrUnhosted for another, ErrHostBuilt after Bind.
func (h *PairHost[P]) Add(b plugin.Rule) error {
	p, ok := b.(*pair[P])
	if !ok {
		return fmt.Errorf("%w: %T", plugin.ErrUnhosted, b)
	}
	if h.bound {
		return fmt.Errorf("%w: %T", plugin.ErrHostBuilt, b)
	}
	p.fa, p.fb = h.familyOf(p.a), h.familyOf(p.b)
	h.pairs = append(h.pairs, p)
	return nil
}

// Empty reports a host with nothing to run.
func (h *PairHost[P]) Empty() bool { return len(h.pairs) == 0 }

// familyOf is the index of t's family among the host's, adding it on first sight; -1 for Any.
func (h *PairHost[P]) familyOf(t tagged) int {
	if t.family == nil {
		return -1
	}
	for i, known := range h.families {
		if known.family() == t.family {
			return i
		}
	}
	if len(h.families) == plugin.MaxFamilies {
		panic(fmt.Sprintf("host: pair rules name more than %d tag families", plugin.MaxFamilies))
	}
	h.families = append(h.families, t.make())
	h.kinds = append(h.kinds, t.family)
	return len(h.families) - 1
}

// Bind adds every family to each of the host's queries — call once, before they are built.
func (h *PairHost[P]) Bind(queries ...*goke.QueryBuilder) {
	h.bound = true
	for _, f := range h.families {
		f.bind(queries)
	}
}

// InChunk is what the i-th entity of the chunk being walked on query carries.
func (h *PairHost[P]) InChunk(query int, cursor *goke.Cursor, i int) plugin.Marks {
	var words [plugin.MaxFamilies]uint64
	for k, f := range h.families {
		words[k] = f.inChunk(query, cursor, i)
	}
	return plugin.MarksOf(words, &h.kinds)
}

// At is what the entity just sought on query carries.
func (h *PairHost[P]) At(query int, cursor *goke.Cursor) plugin.Marks {
	var words [plugin.MaxFamilies]uint64
	for k, f := range h.families {
		words[k] = f.at(query, cursor)
	}
	return plugin.MarksOf(words, &h.kinds)
}

// fits reports whether an entity carrying m satisfies side t of a pair, family index f.
func fits(m plugin.Marks, f int, t tagged) bool {
	return f < 0 || m.Word(f)&(1<<t.bit) != 0
}

// Dispatch runs every rule whose first tag self carries and second tag other carries.
func (h *PairHost[P]) Dispatch(t plugin.Tick, self, other plugin.Marks, pair P) {
	for _, b := range h.pairs {
		if fits(self, b.fa, b.a) && fits(other, b.fb, b.b) {
			b.react(t, pair)
		}
	}
}

// DispatchGrouped is Dispatch for one Self against many Others, run even when none match.
func (h *PairHost[P]) DispatchGrouped(t plugin.Tick, self plugin.Marks, others []plugin.Marks, build func(matched []int) P) {
	for _, b := range h.pairs {
		if !fits(self, b.fa, b.a) {
			continue
		}
		h.matched = h.matched[:0]
		for i := range others {
			if fits(others[i], b.fb, b.b) {
				h.matched = append(h.matched, i)
			}
		}
		b.react(t, build(h.matched))
	}
}

// DispatchEitherWay is Dispatch for a pair with no direction, run whichever way the tags fit.
func (h *PairHost[P]) DispatchEitherWay(t plugin.Tick, a, b plugin.Marks, forward, backward P) {
	for _, p := range h.pairs {
		if fits(a, p.fa, p.a) && fits(b, p.fb, p.b) {
			p.react(t, forward)
			if p.same {
				continue
			}
		}
		if fits(b, p.fa, p.a) && fits(a, p.fb, p.b) {
			p.react(t, backward)
		}
	}
}
