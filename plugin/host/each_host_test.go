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
