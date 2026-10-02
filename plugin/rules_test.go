package plugin_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
)

// moment is a made-up host's description of one entity: its place in the chunk.
type moment struct{ i int }

// Rules over one component share its column in the host's query, the host's own among them:
// two over hostBody and the host reading hostBody itself build one query and each sees every entity.
func TestRules_TriggersOverOneComponentShareItsColumn(t *testing.T) {
	h := &plugin.Rules[moment]{}
	seen := [2]int{}
	for k := range seen {
		if err := h.Add(eachRule(func(_ plugin.Tick, b *hostBody, _ moment) { seen[k] += b.N })); err != nil {
			t.Fatal(err)
		}
	}
	var own goke.OptComp[hostBody]
	plugin.Own(h, &own)
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
			h.Run(plugin.Tick{Dt: time.Millisecond}, cur, func(i int) moment { return moment{i: i} })
		}
	}})
	if seen != [2]int{3, 3} || read != 3 {
		t.Errorf("the rules saw %v, the host read %d; want both rules and the host on all 3", seen, read)
	}
}

// RunWhere hands the moment to the entities keep lets through alone, rules over a component and
// rules over every entity alike.
func TestRules_RunWhereSkipsWhatKeepLeavesOut(t *testing.T) {
	h := &plugin.Rules[moment]{}
	var each, every []int
	if err := h.Add(eachRule(func(_ plugin.Tick, _ *hostBody, m moment) { each = append(each, m.i) })); err != nil {
		t.Fatal(err)
	}
	if err := h.Add(everyRule(func(_ plugin.Tick, m moment) { every = append(every, m.i) })); err != nil {
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
			h.RunWhere(plugin.Tick{}, q.Cursor(), func(i int) bool { return i%2 == 1 }, func(i int) moment { return moment{i: i} })
		}
	}})
	if len(each) != 2 || each[0] != 1 || each[1] != 3 || len(every) != 2 || every[0] != 1 || every[1] != 3 {
		t.Errorf("Each saw %v, Every %v; want both [1 3]", each, every)
	}
}

// eachRule, everyRule and pairRule make rules of Go functions for these tests, as rule.On's are
// made for the rule-driven systems: over a component, over every entity, over pairs.
func eachRule[T, P any](react func(plugin.Tick, *T, P)) any { return &eachOf[T, P]{react: react} }

func everyRule[P any](react func(plugin.Tick, P)) any { return &everyOf[P]{react: react} }

func pairRule[FA, FB, P any](a tag.Tag[FA], b tag.Tag[FB], react func(plugin.Tick, P)) any {
	return &pairOf[P]{a: plugin.SideOf(a), b: plugin.SideOf(b), react: react}
}

type eachOf[T, P any] struct {
	col   *goke.OptComp[T]
	react func(plugin.Tick, *T, P)
}

func (e *eachOf[T, P]) BindColumns(cols *plugin.Columns) { e.col = cols.Of[T]() }

func (e *eachOf[T, P]) RunEach(t plugin.Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P) {
	if !e.col.Present(cursor) {
		return
	}
	ts := e.col.Slice(cursor)
	for i := range cursor.IDs {
		if keep == nil || keep(i) {
			e.react(t, &ts[i], about(i))
		}
	}
}

type everyOf[P any] struct{ react func(plugin.Tick, P) }

func (*everyOf[P]) BindColumns(*plugin.Columns) {}

func (e *everyOf[P]) RunEach(t plugin.Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P) {
	for i := range cursor.IDs {
		if keep == nil || keep(i) {
			e.react(t, about(i))
		}
	}
}

func (e *everyOf[P]) RunOnce(t plugin.Tick, about P) { e.react(t, about) }

type pairOf[P any] struct {
	a, b  plugin.Side
	react func(plugin.Tick, P)
}

func (p *pairOf[P]) PairSides() (self, other plugin.Side) { return p.a, p.b }

func (p *pairOf[P]) RunPair(t plugin.Tick, pair P) { p.react(t, pair) }
