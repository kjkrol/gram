package rule

import (
	"reflect"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
)

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
