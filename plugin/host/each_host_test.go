package host_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
)

// moment is a made-up host's description of one entity: its place in the chunk.
type moment struct{ i int }

// Rules over one component share its column in the host's query, the host's own among them:
// two over body and the host reading body itself build one query and each sees every entity.
func TestEachHost_TriggersOverOneComponentShareItsColumn(t *testing.T) {
	h := &host.EachHost[moment]{}
	seen := [2]int{}
	for k := range seen {
		if err := h.Add(host.Each(func(_ plugin.Tick, b *body, _ moment) { seen[k] += b.N })); err != nil {
			t.Fatal(err)
		}
	}
	var own goke.OptComp[body]
	host.Own(h, &own)
	var read int
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var bodies goke.Comp[body]
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
func TestEachHost_RunWhereSkipsWhatKeepLeavesOut(t *testing.T) {
	h := &host.EachHost[moment]{}
	var each, every []int
	if err := h.Add(host.Each(func(_ plugin.Tick, _ *body, m moment) { each = append(each, m.i) })); err != nil {
		t.Fatal(err)
	}
	if err := h.Add(host.Every(func(_ plugin.Tick, m moment) { every = append(every, m.i) })); err != nil {
		t.Fatal(err)
	}
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var bodies goke.Comp[body]
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
