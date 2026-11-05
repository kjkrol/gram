package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

const enemyName = "enemy"

// how an aggregate stands as the sectors see it: the Keen it carries, nil with none, the level
// its sector is ticked at, the mark it carries, and whether navigation keeps it keen
type keenSeen struct {
	keen   *sector.Keen
	level  lod.Level
	marked lod.Level
	sought bool
}

func TestKeen_AnAggregateOrderedAgainstStaysAtL1TillTheLastOrderEnds(t *testing.T) {
	l0 := sector.Keen{Level: lod.L0}
	l1 := sector.Keen{Level: lod.L1}
	l3 := sector.Keen{Level: lod.L3}
	for _, c := range []struct {
		name string
		own  *sector.Keen
		// the Keen and the level ticked with no order aimed at it and while one is
		alone, aimed *sector.Keen
		calm, kept   lod.Level
	}{
		{name: "none of its own", alone: nil, aimed: &l1, calm: lod.L2, kept: lod.L1},
		{name: "its own at L0", own: &l0, alone: &l0, aimed: &l0, calm: lod.L0, kept: lod.L0},
		{name: "its own at L3", own: &l3, alone: &l3, aimed: &l1, calm: lod.L2, kept: lod.L1},
	} {
		t.Run(c.name, func(t *testing.T) {
			const cols, rows = 96, 16
			g := grid.DefaultGrids{}.Square(cols, rows, legCellSize)
			walker := func(x uint32, speed float32) navUnit {
				at := g.At(x, 8)
				profile := steering.Steering{MaxSpeed: speed * float32(legCellSize)}
				return navUnit{box: cellBox(g, at, legEntitySize), at: at, profile: &profile}
			}
			spec := kind.Spec{comp.Const(world.Aggregate{})}
			if c.own != nil {
				spec = append(spec, comp.Const(*c.own))
			}
			enemy := g.At(80, 8)
			// the aggregate; one sent from near it, who comes there; one from far, slow
			units := []navUnit{
				{box: cellBox(g, enemy, legEntitySize), at: enemy, name: enemyName, spec: spec},
				walker(76, 4),
				walker(10, 1),
			}
			var q *goke.Query
			var base goke.Comp[world.Base]
			var orders goke.OptComp[MoveOrder]
			var keens goke.OptComp[sector.Keen]
			var sought goke.OptComp[order.Sought]
			var m1 goke.OptComp[sector.L1]
			var m2 goke.OptComp[sector.L2]
			var m3 goke.OptComp[sector.L3]
			nw := newNavWorldIn(t, nil, cols, rows, legCellSize, units,
				world.SectorsCfg{Buckets: 2}, abstractLadder,
				func(w *world.Plugin, _ *board.Plugin) { w.Write().Observe(&observed{}) },
				goke.SystemFn{OnInit: func(si *goke.SysInit) {
					q = si.NewQueryBuilder(&base).Optional(&orders, &keens, &sought).
						Optional(&m1, &m2, &m3).Build()
				}})
			w := nw.nav.worldPlugin
			seen := func() keenSeen {
				t.Helper()
				if !q.Seek(nw.ids[0]) {
					t.Fatal("the aggregate is gone")
				}
				cur := q.Cursor()
				at := base.At(cur).Pos.Center()
				s := keenSeen{level: w.Read(0).LevelAt(at.X, at.Y), sought: sought.Present(cur)}
				if k := keens.At(cur); k != nil {
					s.keen = k
				}
				switch {
				case m1.Present(cur):
					s.marked = lod.L1
				case m2.Present(cur):
					s.marked = lod.L2
				case m3.Present(cur):
					s.marked = lod.L3
				}
				return s
			}
			ordered := func(k int) bool {
				t.Helper()
				if !q.Seek(nw.ids[k]) {
					t.Fatalf("unit %d is gone", k)
				}
				return orders.At(q.Cursor()) != nil
			}
			check := func(when string, keen *sector.Keen, level lod.Level, kept bool) {
				t.Helper()
				s := seen()
				sameKeen := (s.keen == nil) == (keen == nil) && (keen == nil || *s.keen == *keen)
				if !sameKeen || s.level != level || s.marked != level || s.sought != kept {
					t.Errorf("%s: Keen %v at %v marked %v, kept %v; want Keen %v at %v, kept %v",
						when, s.keen, s.level, s.marked, s.sought, keen, level, kept)
				}
			}
			run := func(ticks int) {
				for range ticks {
					nw.ecs.Tick(time.Second / 60)
				}
			}
			give := func(k int, cmd MoveTo) {
				t.Helper()
				if !w.Write().Carrier().PutFrom(nw.ids[k], cmd) {
					t.Fatalf("unit %d's %v carried by nobody", k, cmd)
				}
			}

			run(2)
			check("alone", c.alone, c.calm, false)
			give(2, MoveTo{Cell: enemy})
			run(4)
			check("sent at its cell", c.alone, c.calm, false)

			give(1, MoveTo{To: ceremony.Named(enemyName)})
			give(2, MoveTo{To: ceremony.Named(enemyName)})
			run(4)
			check("two sent at it", c.aimed, c.kept, true)
			arrived := false
			for range 10 * 60 {
				if run(1); !ordered(1) {
					arrived = true
					break
				}
			}
			if !arrived || !ordered(2) {
				t.Fatalf("the near one arrived %v, the far one under way %v; want both", arrived,
					ordered(2))
			}
			run(4)
			check("one come", c.aimed, c.kept, true)

			give(2, MoveTo{Cell: g.At(5, 8)})
			run(4)
			check("the other sent elsewhere", c.alone, c.calm, false)
		})
	}
}

func TestKeen_ItsRecordsOutlastASave(t *testing.T) {
	path := t.TempDir() + "/save.bin"
	sought := order.Sought{Keen: sector.Keen{Level: lod.L3}, Had: true}
	aimed := MoveOrder{Target: 7, Aim: 3, Aimed: true}
	ecs := goke.New()
	var record goke.Comp[order.Sought]
	var orders goke.Comp[MoveOrder]
	var aiming goke.Comp[order.Aiming]
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&record)
		f.Create(1)
		f.Next()
		record.Slice(&f.Cursor)[0] = sought
		g := si.NewFactory(&orders, &aiming)
		g.Create(1)
		g.Next()
		orders.Slice(&g.Cursor)[0] = aimed
	}})
	ecs.Pause()
	if err := ecs.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded := goke.New()
	if err := loaded.Load(path, (&module{}).LoadComps()...); err != nil {
		t.Fatalf("Load: %v", err)
	}
	var records []order.Sought
	var aims []MoveOrder
	loaded.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q := si.NewQueryBuilder(&record).Build()
		for q.All(); q.Next(); {
			records = append(records, record.Slice(q.Cursor())...)
		}
		a := si.NewQueryBuilder(&orders).Include(goke.Include[order.Aiming]()).Build()
		for a.All(); a.Next(); {
			aims = append(aims, orders.Slice(a.Cursor())...)
		}
	}})
	if len(records) != 1 || records[0] != sought || len(aims) != 1 || aims[0] != aimed {
		t.Errorf("loaded %v and aiming %+v; want %v and %+v", records, aims, sought, aimed)
	}
}

func TestKeen_AKeenAnAggregateGivesItselfOutlastsTheOrdersAimedAtIt(t *testing.T) {
	l0 := sector.Keen{Level: lod.L0}
	l1 := sector.Keen{Level: lod.L1}
	for _, c := range []struct {
		name string
		// the Keen it gives itself before an order is aimed at it and while one is, nil for none
		before, during *sector.Keen
		// whether it calms while one is
		calms bool
		want  *sector.Keen
	}{
		{name: "given before", before: &l1, want: &l1},
		{name: "given while aimed at", during: &l0, want: &l0},
		{name: "calmed while aimed at", before: &l1, calms: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			const cols, rows = 96, 16
			g := grid.DefaultGrids{}.Square(cols, rows, legCellSize)
			enemy, from := g.At(80, 8), g.At(76, 8)
			profile := steering.Steering{MaxSpeed: 4 * float32(legCellSize)}
			units := []navUnit{
				{box: cellBox(g, enemy, legEntitySize), at: enemy, name: enemyName,
					spec: kind.Spec{comp.Const(world.Aggregate{})}},
				{box: cellBox(g, from, legEntitySize), at: from, profile: &profile},
			}
			var q *goke.Query
			var base goke.Comp[world.Base]
			var orders goke.OptComp[MoveOrder]
			var keens goke.OptComp[sector.Keen]
			nw := newNavWorldIn(t, nil, cols, rows, legCellSize, units,
				world.SectorsCfg{Buckets: 2}, abstractLadder,
				func(w *world.Plugin, _ *board.Plugin) { w.Write().Observe(&observed{}) },
				goke.SystemFn{OnInit: func(si *goke.SysInit) {
					q = si.NewQueryBuilder(&base).Optional(&orders, &keens).Build()
				}})
			w := nw.nav.worldPlugin
			run := func(ticks int) {
				for range ticks {
					nw.ecs.Tick(time.Second / 60)
				}
			}
			give := func(k int, cmd command.Command) {
				t.Helper()
				if !w.Write().Carrier().PutFrom(nw.ids[k], cmd) {
					t.Fatalf("unit %d's %v carried by nobody", k, cmd)
				}
			}
			keen := func() *sector.Keen {
				t.Helper()
				if !q.Seek(nw.ids[0]) {
					t.Fatal("the aggregate is gone")
				}
				return keens.At(q.Cursor())
			}

			run(2)
			if c.before != nil {
				give(0, world.Keen{Level: c.before.Level})
				run(2)
			}
			give(1, MoveTo{To: ceremony.Named(enemyName)})
			run(4)
			if c.during != nil {
				give(0, world.Keen{Level: c.during.Level})
			}
			if c.calms {
				give(0, world.Calm{})
			}
			run(4)
			if !q.Seek(nw.ids[1]) || orders.At(q.Cursor()) == nil {
				t.Fatal("the one sent at the aggregate has come before the Keen was given")
			}
			for range 10 * 60 {
				if run(1); q.Seek(nw.ids[1]) && orders.At(q.Cursor()) == nil {
					break
				}
			}
			run(4)
			if got := keen(); (got == nil) != (c.want == nil) || got != nil && *got != *c.want {
				t.Errorf("the order aimed at it over, it carries Keen %v, want %v", got, c.want)
			}
		})
	}
}
