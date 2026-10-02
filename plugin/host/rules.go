package host

import (
	"reflect"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
)

// Pair is a rule for every pair a host meets where one entity carries a and the other b;
// tag.Any on a side takes whatever is there. A plugin wraps it as its own Between.
func Pair[FA, FB, P any](a tag.Tag[FA], b tag.Tag[FB], react func(t plugin.Tick, pair P)) plugin.Rule {
	return &pair[P]{
		a: tagOf(a), b: tagOf(b),
		same:  reflect.TypeFor[FA]() == reflect.TypeFor[FB]() && uint8(a) == uint8(b),
		react: react,
	}
}

// Each is a rule run on every entity a host visits that carries T; T must not be a component
// the host already requires of every entity — Every is for those. A plugin wraps it as its own Each.
func Each[T, P any](react func(t plugin.Tick, state *T, about P)) plugin.Rule {
	return &each[T, P]{react: react}
}

// Every is a rule run on every entity a host visits, with no state component of its own.
func Every[P any](react func(t plugin.Tick, about P)) plugin.Rule {
	return &every[P]{react: react}
}

// tagged is one side of a pair with its family erased: which family, which bit, and how to
// build the family's probe, since a host cannot from the type alone.
type tagged struct {
	family reflect.Type // nil for Any
	bit    uint8
	make   func() tagProbe
}

func tagOf[F any](t tag.Tag[F]) tagged {
	if reflect.TypeFor[F]() == reflect.TypeFor[tag.Anything]() {
		return tagged{}
	}
	return tagged{family: reflect.TypeFor[F](), bit: uint8(t), make: func() tagProbe { return &probe[F]{} }}
}

// tagProbe reads one family's Tags on each of a host's queries: by chunk where one is being
// walked, by entity where one was sought.
type tagProbe interface {
	family() reflect.Type
	bind(queries []*goke.QueryBuilder)
	inChunk(query int, cursor *goke.Cursor, i int) uint64
	at(query int, cursor *goke.Cursor) uint64
}

type probe[F any] struct {
	comps []goke.OptComp[tag.Tags[F]] // one per host query; never grown once bound
}

func (p *probe[F]) family() reflect.Type { return reflect.TypeFor[F]() }

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
