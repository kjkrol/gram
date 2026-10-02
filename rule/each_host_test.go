package rule

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
)

// moment is a made-up host's description of one entity: its place in the chunk.
type moment struct{ i int }

// Rules over one component share its column in the host's query, the host's own among them:
// two over hostBody and the host reading hostBody itself build one query and each sees every entity.
func TestEachHost_TriggersOverOneComponentShareItsColumn(t *testing.T) {
	h := &EachHost[moment]{}
	seen := [2]int{}
	for k := range seen {
		if err := h.Add(eachRule(func(_ Tick, b *hostBody, _ moment) { seen[k] += b.N })); err != nil {
			t.Fatal(err)
		}
	}
	var own goke.OptComp[hostBody]
	Own(h, &own)
	var read int
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var bodies goke.Comp[hostBody]
		f := si.NewFactory(&bodies)
		f.Create(3)
		f.Next()
		for i := range bodies.Slice(&f.Cursor) {
			bodies.Slice(&f.Cursor)[i].N = 1
		}
		qb := si.NewQueryBuilder()
		h.Bind(qb)
		q := qb.Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			read += len(own.Slice(cur))
			h.Run(Tick{Dt: time.Millisecond}, cur, func(i int) moment { return moment{i: i} })
		}
	}})
	if seen != [2]int{3, 3} || read != 3 {
		t.Errorf("the rules saw %v, the host read %d; want both rules and the host on all 3", seen, read)
	}
}

// RunWhere hands the moment to the entities keep lets through alone, rules over a component and
// rules over every entity alike.
func TestEachHost_RunWhereSkipsWhatKeepLeavesOut(t *testing.T) {
	h := &EachHost[moment]{}
	var each, every []int
	if err := h.Add(eachRule(func(_ Tick, _ *hostBody, m moment) { each = append(each, m.i) })); err != nil {
		t.Fatal(err)
	}
	if err := h.Add(everyRule(func(_ Tick, m moment) { every = append(every, m.i) })); err != nil {
		t.Fatal(err)
	}
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var bodies goke.Comp[hostBody]
		f := si.NewFactory(&bodies)
		f.Create(4)
		f.Next()
		qb := si.NewQueryBuilder()
		h.Bind(qb)
		q := qb.Build()
		for q.All(); q.Next(); {
			h.RunWhere(Tick{}, q.Cursor(), func(i int) bool { return i%2 == 1 }, func(i int) moment { return moment{i: i} })
		}
	}})
	if len(each) != 2 || each[0] != 1 || each[1] != 3 || len(every) != 2 || every[0] != 1 || every[1] != 3 {
		t.Errorf("Each saw %v, Every %v; want both [1 3]", each, every)
	}
}

// pairRule, eachRule and everyRule make a rule of a Go function, as the hosts run them: for the
// hosts' own tests.
func pairRule[FA, FB, P any](a tag.Tag[FA], b tag.Tag[FB], react func(Tick, P)) Rule {
	return pairOf[P](pairSideOf(a), pairSideOf(b), react)
}

func eachRule[T, P any](react func(Tick, *T, P)) Rule {
	return newEachWith[P](stateOf[T](), func(t Tick, state any, about P) { react(t, state.(*T), about) })
}

func everyRule[P any](react func(Tick, P)) Rule { return &every[P]{react: react} }
