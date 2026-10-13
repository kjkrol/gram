package plugin

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
)

// Side is one side of a pair rule: a tag of a family, or anybody.
type Side struct {
	family reflect.Type // nil: anybody
	bit    uint8
	probe  func() familyProbe
}

// Anybody reports a side of no tag, which anybody is.
func (s Side) Anybody() bool { return s.family == nil }

// SideOf is t as a side of a pair rule; tag.Any is anybody.
func SideOf[F any](t tag.Tag[F]) Side {
	if reflect.TypeFor[F]() == reflect.TypeFor[tag.Anything]() {
		return Side{}
	}
	return Side{family: reflect.TypeFor[F](), bit: uint8(t), probe: func() familyProbe { return &probe[F]{} }}
}

type pairRule[P any] interface {
	PairSides() (self, other Side)
	RunPair(t Tick, pair P)
}

type withinRule interface{ PairWithin() []Side }

type paired[P any] struct {
	rule   pairRule[P]
	a, b   Side
	fa, fb int // the families' places, -1 for anybody
	same   bool
	within []Side
	fw     []int
}

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

// PairRules are the rules of a moment P of two entities a plugin runs in its pass: it reads the
// tag families the rules name, eight at most, off the entities (InChunk, At) and dispatches each
// pair to the rules whose sides it carries.
type PairRules[P any] struct {
	families []familyProbe
	kinds    []reflect.Type
	rules    []paired[P]
	bound    bool
	matched  []int // scratch
}

// Add takes a pair rule of P; ErrUnhosted for another, ErrHostBuilt after Bind.
func (r *PairRules[P]) Add(rule any) error {
	rule = hosted(rule)
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
			if s.family == b.family && s.bit == b.bit {
				held.same = true // the rule's own side is the other's too: once a pair
			}
		}
	}
	r.rules = append(r.rules, held)
	return nil
}

// Empty reports whether no rule was added.
func (r *PairRules[P]) Empty() bool { return len(r.rules) == 0 }

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

// Bind adds every family the rules name to each of the system's queries. Call it once, before
// they are built.
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

// DispatchGrouped is Dispatch for one entity against many others, run with none matched too
// unless the rule names the other's side; build makes the moment of the others matched, by
// their places in others.
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
		if len(r.matched) == 0 && p.fb >= 0 {
			continue
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

type familyProbe interface {
	bind(queries []*goke.QueryBuilder)
	inChunk(query int, cursor *goke.Cursor, i int) uint64
	at(query int, cursor *goke.Cursor) uint64
}

type probe[F any] struct {
	comps []goke.OptComp[tag.Tags[F]] // one a query; never grown once bound
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
