package plugin

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
)

// Side is one side of a pair rule — a tag of a family, or anybody — as PairRules reads it.
type Side struct {
	family reflect.Type // nil: anybody
	bit    uint8
	probe  func() familyProbe
}

// SideOf is t as a side of a pair rule; tag.Any takes whatever is there.
func SideOf[F any](t tag.Tag[F]) Side {
	if reflect.TypeFor[F]() == reflect.TypeFor[tag.Anything]() {
		return Side{}
	}
	return Side{family: reflect.TypeFor[F](), bit: uint8(t), probe: func() familyProbe { return &probe[F]{} }}
}

// pairRule is a rule of a moment of two, as rule.On builds it: what PairRules dispatches.
type pairRule[P any] interface {
	PairSides() (self, other Side)
	RunPair(t Tick, pair P)
}

// withinRule is a pair rule whose entity must carry more tags than its side's: a role's
// (rule.Part.Obeys).
type withinRule interface{ PairWithin() []Side }

// paired is a pair rule as PairRules holds it: its sides and their families' places, -1 for
// anybody.
type paired[P any] struct {
	rule   pairRule[P]
	a, b   Side
	fa, fb int
	same   bool
	within []Side // more tags the first side carries
	fw     []int  // their families' places
}

// first reports whether an entity carrying m is the rule's first side: its tag and every one it
// must carry besides.
func (p *paired[P]) first(m Marks) bool {
	if !fits(m, p.fa, p.a) {
		return false
	}
	for k, s := range p.within {
		if !fits(m, p.fw[k], s) {
			return false
		}
	}
	return true
}

// PairRules are the rules of a moment P of two entities — a collision.Meeting, a
// vision.Sighting — a rule-driven system runs in its pass: it reads every tag family the rules
// name, eight at most, off the entities it walks or seeks (InChunk, At) and dispatches each pair
// to the rules whose sides it carries.
type PairRules[P any] struct {
	families []familyProbe
	kinds    []reflect.Type // the families' types, in the same order
	rules    []paired[P]
	bound    bool
	matched  []int // DispatchGrouped's scratch
}

// Add takes a pair rule of P made by rule.On; ErrUnhosted for another, ErrHostBuilt after Bind.
func (r *PairRules[P]) Add(rule any) error {
	p, ok := rule.(pairRule[P])
	if !ok {
		return fmt.Errorf("%w: %v", ErrUnhosted, rule)
	}
	if r.bound {
		return fmt.Errorf("%w: %v", ErrHostBuilt, rule)
	}
	a, b := p.PairSides()
	held := paired[P]{rule: p, a: a, b: b, fa: r.familyOf(a), fb: r.familyOf(b),
		same: a.family == b.family && a.bit == b.bit}
	if w, ok := rule.(withinRule); ok {
		for _, s := range w.PairWithin() {
			held.within = append(held.within, s)
			held.fw = append(held.fw, r.familyOf(s))
		}
	}
	r.rules = append(r.rules, held)
	return nil
}

// Empty reports whether no rule was added.
func (r *PairRules[P]) Empty() bool { return len(r.rules) == 0 }

// familyOf is the place of s's family among the system's, added on first sight; -1 for anybody.
func (r *PairRules[P]) familyOf(s Side) int {
	if s.family == nil {
		return -1
	}
	for i, known := range r.kinds {
		if known == s.family {
			return i
		}
	}
	if len(r.families) == maxFamilies {
		panic(fmt.Sprintf("plugin: pair rules name more than %d tag families", maxFamilies))
	}
	r.families = append(r.families, s.probe())
	r.kinds = append(r.kinds, s.family)
	return len(r.families) - 1
}

// Bind adds every family the rules name to each of the system's queries; call once, before they
// are built.
func (r *PairRules[P]) Bind(queries ...*goke.QueryBuilder) {
	r.bound = true
	for _, f := range r.families {
		f.bind(queries)
	}
}

// InChunk is what the i-th entity of the chunk being walked on query carries.
func (r *PairRules[P]) InChunk(query int, cursor *goke.Cursor, i int) Marks {
	var words [maxFamilies]uint64
	for k, f := range r.families {
		words[k] = f.inChunk(query, cursor, i)
	}
	return Marks{words: words}
}

// At is what the entity just sought on query carries.
func (r *PairRules[P]) At(query int, cursor *goke.Cursor) Marks {
	var words [maxFamilies]uint64
	for k, f := range r.families {
		words[k] = f.at(query, cursor)
	}
	return Marks{words: words}
}

// fits reports whether an entity carrying m is side s, its family at f.
func fits(m Marks, f int, s Side) bool {
	return f < 0 || m.words[f]&(1<<s.bit) != 0
}

// Dispatch runs every rule whose first side self carries and second side other carries.
func (r *PairRules[P]) Dispatch(t Tick, self, other Marks, pair P) {
	for _, p := range r.rules {
		if p.first(self) && fits(other, p.fb, p.b) {
			p.rule.RunPair(t, pair)
		}
	}
}

// DispatchGrouped is Dispatch for one entity against many others, run even when none match:
// build makes the moment of the others matched, by their places in others.
func (r *PairRules[P]) DispatchGrouped(t Tick, self Marks, others []Marks, build func(matched []int) P) {
	for _, p := range r.rules {
		if !p.first(self) {
			continue
		}
		r.matched = r.matched[:0]
		for i := range others {
			if fits(others[i], p.fb, p.b) {
				r.matched = append(r.matched, i)
			}
		}
		p.rule.RunPair(t, build(r.matched))
	}
}

// DispatchEitherWay is Dispatch for a pair with no direction, run whichever way the sides fit.
func (r *PairRules[P]) DispatchEitherWay(t Tick, a, b Marks, forward, backward P) {
	for _, p := range r.rules {
		if p.first(a) && fits(b, p.fb, p.b) {
			p.rule.RunPair(t, forward)
			if p.same {
				continue
			}
		}
		if p.first(b) && fits(a, p.fb, p.b) {
			p.rule.RunPair(t, backward)
		}
	}
}

// familyProbe reads one family's Tags on each of a system's queries: by chunk where one is being
// walked, by entity where one was sought.
type familyProbe interface {
	bind(queries []*goke.QueryBuilder)
	inChunk(query int, cursor *goke.Cursor, i int) uint64
	at(query int, cursor *goke.Cursor) uint64
}

type probe[F any] struct {
	comps []goke.OptComp[tag.Tags[F]] // one per query; never grown once bound
}

func (p *probe[F]) bind(queries []*goke.QueryBuilder) {
	p.comps = make([]goke.OptComp[tag.Tags[F]], len(queries))
	for i, qb := range queries {
		qb.Optional(&p.comps[i])
	}
}

func (p *probe[F]) inChunk(query int, cursor *goke.Cursor, i int) uint64 {
	if !p.comps[query].Present(cursor) {
		return 0
	}
	return uint64(p.comps[query].Slice(cursor)[i])
}

func (p *probe[F]) at(query int, cursor *goke.Cursor) uint64 {
	if v := p.comps[query].At(cursor); v != nil {
		return uint64(*v)
	}
	return 0
}
