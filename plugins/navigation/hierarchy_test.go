package navigation

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// withHierarchy is a rig's prepare having the navigation made over the world plan the long
// routes as h says
func withHierarchy(h Hierarchy) func(*world.Plugin, *board.Plugin) {
	return func(w *world.Plugin, _ *board.Plugin) {
		for _, p := range w.Made().All() {
			if nav, ok := p.(*Plugin); ok {
				nav.WithHierarchy(h)
			}
		}
	}
}

// a unit of the climbs' rigs: where it stands and how wide its box is
type climber struct {
	at   cell.ID
	side uint32
}

// climbers stand at starts, boxes of 12 as the groups' units
func climbers(starts []cell.ID) []climber {
	out := make([]climber, len(starts))
	for i, c := range starts {
		out[i] = climber{at: c, side: 12}
	}
	return out
}

// withCache is a rig's prepare having the navigation made over the world keep its hierarchies
// as c says
func withCache(c Cache) func(*world.Plugin, *board.Plugin) {
	return func(w *world.Plugin, _ *board.Plugin) {
		for _, p := range w.Made().All() {
			if nav, ok := p.(*Plugin); ok {
				nav.WithCache(c)
			}
		}
	}
}

// newRig is a board of side cells a side, each of cellSize, the units standing as given, laid by
// lay, the plugin planning the long routes as h says and prepared further by prepares
func newRig(t testing.TB, pool *parallel.Pool, h Hierarchy, side, cellSize uint32,
	units []climber, lay func(nw *navWorld),
	prepares ...func(*world.Plugin, *board.Plugin)) *groupRig {
	t.Helper()
	g := grid.DefaultGrids{}.Square(side, side, cellSize)
	placed := make([]navUnit, len(units))
	for i, u := range units {
		placed[i] = navUnit{box: cellBox(g, u.at, u.side), at: u.at, order: &MoveOrder{}}
	}
	r := &groupRig{}
	prepare := func(w *world.Plugin, b *board.Plugin) {
		withHierarchy(h)(w, b)
		for _, p := range prepares {
			p(w, b)
		}
	}
	r.nw = newNavWorldIn(t, pool, side, side, cellSize, placed, world.SectorsCfg{}, lod.Ladder{},
		prepare, goke.SystemFn{OnInit: func(si *goke.SysInit) {
			r.ordered = si.NewQueryBuilder(&r.orders, &r.at).Build()
		}})
	r.coord = r.nw.nav.module.navigationSystem.Routing.(*routing.Coordinator)
	lay(r.nw)
	return r
}

// newClimbRig is the groups' board, its wall with two gaps, the units standing as given
func newClimbRig(t testing.TB, pool *parallel.Pool, h Hierarchy, units []climber) *groupRig {
	t.Helper()
	return newRig(t, pool, h, groupSide, groupCell, units, func(nw *navWorld) {
		for y := range uint32(groupSide) {
			if y != 8 && y != 24 {
				nw.board.Set(nw.grid.At(16, y), wall)
			}
		}
	})
}

// the routes' board of the routing benchmarks
func longGrid() grid.Grid { return grid.DefaultGrids{}.Square(routeSide, routeSide, routeCell) }

// newLongRig is the routes' board (layRoutes), the units standing as given, the cells round them
// and round keep clear
func newLongRig(t testing.TB, pool *parallel.Pool, h Hierarchy, units []climber,
	keep ...cell.ID) *groupRig {
	t.Helper()
	return newKeptLongRig(t, pool, h, Cache{}, units, keep...)
}

// newKeptLongRig is newLongRig with the hierarchies kept as c says
func newKeptLongRig(t testing.TB, pool *parallel.Pool, h Hierarchy, c Cache, units []climber,
	keep ...cell.ID) *groupRig {
	t.Helper()
	for _, u := range units {
		keep = append(keep, u.at)
	}
	return newRig(t, pool, h, routeSide, routeCell, units, func(nw *navWorld) {
		layRoutes(nw, keep)
	}, withCache(c))
}

// sendEach gives each unit a MoveTo of its own to goal, as send does
func (r *groupRig) sendEach(goal cell.ID, units ...int) {
	for _, u := range units {
		r.send(uint32(100+u), goal, u)
	}
}

// built has the routing Update, nothing asked, till the hierarchies it keeps are built, and
// reports the clusters built so far
func (r *groupRig) built(t testing.TB) uint64 {
	t.Helper()
	for range 1000 {
		if r.update(); !r.coord.Building() {
			return r.coord.Builds()
		}
	}
	t.Fatal("the hierarchies never built")
	return 0
}

// the units' routes, by their starts' order
func (r *groupRig) paths() []Path {
	var out []Path
	for _, id := range r.nw.ids {
		o, _ := r.order(id)
		out = append(out, o.Path)
	}
	return out
}

func samePaths(t *testing.T, got, want []Path, what string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d routes %s, want %d", len(got), what, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("unit %d: route %v %s, want %v", i, got[i].Steps[:got[i].Length], what,
				want[i].Steps[:want[i].Length])
		}
	}
}

// routesAs has the units of a rig planning the long routes as h says each sent to goal, the
// routing answering their asks once
func routesAs(t *testing.T, h Hierarchy, starts []cell.ID, goal cell.ID) (*groupRig, []Path) {
	t.Helper()
	r := newClimbRig(t, nil, h, climbers(starts))
	r.sendEach(goal, every(len(starts))...)
	r.update()
	r.routed(t, every(len(starts))...)
	return r, r.paths()
}

// chunked checks every unit given has a chunk of its route from where it stands, step by step,
// a Path long or to its target
func (r *groupRig) chunked(t *testing.T, units ...int) {
	t.Helper()
	for _, u := range units {
		o, at := r.order(r.nw.ids[u])
		p := o.Path
		if o.Ask != order.AskNone || p.From != at || p.Length == 0 ||
			p.Length < MaxPathLength && p.Steps[p.Length-1] != o.Target {
			t.Errorf("unit %d at %v: ask %v, a chunk from %v of %d steps, want one to %v or "+
				"%d long", u, at, o.Ask, p.From, p.Length, o.Target, MaxPathLength)
			continue
		}
		prev := at
		for _, s := range p.Steps[:p.Length] {
			if _, ok := grid.Way(r.nw.grid, prev, s); !ok || !r.nw.board.Kind(s).Admits(cell.Land) {
				t.Errorf("unit %d: a step %v->%v of its route", u, prev, s)
			}
			prev = s
		}
	}
}

// toggled has the cell c of r's board rock where it was open ground and open where rock
func (r *groupRig) toggled(c cell.ID) {
	if r.nw.board.Kind(c).Admits(cell.Land) {
		r.nw.board.Set(c, cell.Kind{Pace: 1, Solid: true})
	} else {
		r.nw.board.Set(c, cell.Kind{Pace: 1, Allows: cell.Land})
	}
}

func TestNavigation_UseDecidesTheHierarchy(t *testing.T) {
	r := newClimbRig(t, nil, Hierarchy{From: 20}, climbers(westOf(1)))
	if got, want := r.nw.nav.Read(0).Hierarchy(), (Hierarchy{Side: 16, From: 20}); got != want {
		t.Errorf("the hierarchy %+v once used, want %+v: the side of a world without sectors",
			got, want)
	}
}

// the side of the world's sectors in cells where they span whole cells of a square grid, else 16
func TestNavigation_TheHierarchysSideIsTheSectorsWhereTheySpanWholeCells(t *testing.T) {
	for _, c := range []struct {
		name    string
		asked   Hierarchy
		grid    grid.Grid
		sectors world.SectorsCfg
		want    Hierarchy
	}{
		{"sectors of two buckets", Hierarchy{}, grid.DefaultGrids{}.Square(32, 32, 16),
			world.SectorsCfg{Buckets: 2}, Hierarchy{Side: 16, From: 64}},
		{"sectors of four buckets", Hierarchy{}, grid.DefaultGrids{}.Square(32, 32, 16),
			world.SectorsCfg{Buckets: 4}, Hierarchy{Side: 32, From: 64}},
		{"no sectors", Hierarchy{}, grid.DefaultGrids{}.Square(32, 32, 16),
			world.SectorsCfg{}, Hierarchy{Side: 16, From: 64}},
		{"sectors of no whole cells", Hierarchy{}, grid.DefaultGrids{}.Square(32, 32, 24),
			world.SectorsCfg{Buckets: 2}, Hierarchy{Side: 16, From: 64}},
		{"a hex grid", Hierarchy{}, grid.DefaultGrids{}.Hex(32, 32, 8),
			world.SectorsCfg{Buckets: 2}, Hierarchy{Side: 16, From: 64}},
		{"a side asked", Hierarchy{Side: 10}, grid.DefaultGrids{}.Square(32, 32, 16),
			world.SectorsCfg{Buckets: 2}, Hierarchy{Side: 10, From: 64}},
		{"a distance asked", Hierarchy{From: 24}, grid.DefaultGrids{}.Square(32, 32, 16),
			world.SectorsCfg{}, Hierarchy{Side: 16, From: 24}},
		{"off", Hierarchy{Off: true}, grid.DefaultGrids{}.Square(32, 32, 16),
			world.SectorsCfg{Buckets: 2}, Hierarchy{Off: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			// a bucket of 128 by the world's bounds on entities: eight of the largest a side
			w := world.NewPlugin(world.Config{
				Space:    world.SpaceCfg{Width: 512, Height: 512},
				Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 16},
				Heights:  true,
				Sectors:  c.sectors,
			})
			if b := w.Write().Space().BucketSize; b != 128 {
				t.Fatalf("a bucket of %d, want 128", b)
			}
			brd := board.NewPlugin(c.grid, &cell.SingleOccupancy{}, w).Read(0).Board()
			if got := c.asked.resolve(w, brd); got != c.want {
				t.Errorf("%+v resolved as %+v, want %+v", c.asked, got, c.want)
			}
		})
	}
}

// a route shorter than From is searched as it was before navigation kept hierarchies, none built
// for it
func TestRouting_AShortRouteIsSearchedAsBefore(t *testing.T) {
	goal := groupGrid().At(26, 16)
	near, short := routesAs(t, Hierarchy{Side: 8, From: 40}, westOf(4), goal)
	off, exact := routesAs(t, Hierarchy{Off: true}, westOf(4), goal)
	samePaths(t, short, exact, "with a hierarchy kept")
	c := near.coord
	if c.Searches() != off.coord.Searches() || c.Climbs() != 0 || c.Builds() != 0 {
		t.Errorf("%d searches, %d climbs and %d regions built, want the %d searches of no "+
			"hierarchy alone", c.Searches(), c.Climbs(), c.Builds(), off.coord.Searches())
	}
}

// a far route the hierarchy built refuses, its ends nearer than two of its regions, is searched
// as before
func TestRouting_AFarRouteTheHierarchyRefusesIsSearched(t *testing.T) {
	g := groupGrid()
	routes := func(h Hierarchy) (*groupRig, []Path) {
		r := newClimbRig(t, nil, h, climbers(westOf(4)))
		r.sendEach(g.At(26, 16), every(4)...)
		r.update()
		r.built(t)
		r.sendEach(g.At(10, 12), every(4)...)
		r.update()
		r.routed(t, every(4)...)
		return r, r.paths()
	}
	far, refused := routes(Hierarchy{Side: 16, From: 4})
	off, exact := routes(Hierarchy{Off: true})
	samePaths(t, refused, exact, "refused a climb")
	if c := far.coord; c.Searches() != off.coord.Searches() || c.Climbs() != 0 ||
		c.Builds() == 0 {
		t.Errorf("%d searches, %d climbs, %d regions built; want the %d searches of no "+
			"hierarchy, one built", c.Searches(), c.Climbs(), c.Builds(), off.coord.Searches())
	}
}

// with the hierarchy off every route is searched, the far ones too, and none is built
func TestRouting_HierarchyOffPlansEveryRouteExact(t *testing.T) {
	goal := groupGrid().At(28, 28)
	off, offPaths := routesAs(t, Hierarchy{Off: true}, westOf(8), goal)
	if !off.nw.nav.Read(0).Hierarchy().Off {
		t.Error("the hierarchy on, asked off")
	}
	_, exact := routesAs(t, Hierarchy{From: math.MaxUint16}, westOf(8), goal)
	samePaths(t, offPaths, exact, "with the hierarchy off")
	if c := off.coord; c.Searches() != 8 || c.Climbs() != 0 || c.Builds() != 0 {
		t.Errorf("%d searches, %d climbs and %d regions built, want a search a unit alone",
			c.Searches(), c.Climbs(), c.Builds())
	}
}

// the asks of units of five classes for far routes, one hierarchy serving them all, answered on a
// pool's goroutines get the routes and the counts one goroutine alone gives them, Update after
// Update: the hierarchy is expected on the caller's goroutine, in the asks' order
func TestRouting_AnExpectIsDeterministicOnThePool(t *testing.T) {
	run := func(pool *parallel.Pool) ([]Path, [3]uint64) {
		var units []climber
		for i, at := range westOf(10) {
			units = append(units, climber{at: at, side: uint32(4 + 2*(i%5))})
		}
		r := newClimbRig(t, pool, Hierarchy{Side: 8, From: 8}, units)
		g := r.nw.grid
		r.sendEach(g.At(30, 30), every(10)...)
		r.update()
		r.built(t)
		var paths []Path
		for k, goal := range []cell.ID{g.At(28, 28), g.At(26, 4), g.At(24, 16), g.At(29, 9)} {
			asked := every(10)
			if k%2 == 1 {
				asked = []int{4, 9, 0, 5, 3}
			}
			r.sendEach(goal, asked...)
			r.update()
			r.routed(t, asked...)
			paths = append(paths, r.paths()...)
			r.built(t)
		}
		return paths, [3]uint64{r.coord.Searches(), r.coord.Climbs(), r.coord.Builds()}
	}
	one, oneCounts := run(nil)
	many, manyCounts := run(parallel.New(4))
	samePaths(t, many, one, "on the pool")
	if manyCounts != oneCounts {
		t.Errorf("searches, climbs and builds %v on the pool, %v on one goroutine", manyCounts,
			oneCounts)
	}
	if oneCounts[1] == 0 {
		t.Error("no route climbed")
	}
}

// a sway turns the board's Version and leaves its Passage, the hierarchies as they are; a cell
// touched has its region built again
func TestRouting_ASweepStalesOnPassageAlone(t *testing.T) {
	wr := newWeatheredRig(t, westOf(3))
	wr.coord.Hierarchy(8, 8)
	wr.sendEach(wr.nw.grid.At(26, 16), every(3)...)
	wr.update()
	builds := wr.built(t)
	if builds == 0 {
		t.Fatal("no region built for the far routes")
	}
	wr.sways(t, wr.nw.grid.At(10, 7))
	wr.update()
	if built := wr.coord.Builds() - builds; built != 0 {
		t.Errorf("%d regions built again under the sway, want none", built)
	}
	wr.nw.board.Touch(wr.nw.grid.At(10, 7))
	wr.update()
	if wr.coord.Builds() == builds {
		t.Error("a cell touched, no region built again")
	}
}

// an Update climbing the far routes of units of two classes allocates nothing once warm: the
// hierarchy kept, nor where the board changed under it
func TestRouting_AnUpdateWithHierarchiesAllocatesNothing(t *testing.T) {
	units := climbers(westOf(8))
	for i := range units {
		units[i].side -= uint32(6 * (i % 2))
	}
	r := newClimbRig(t, nil, Hierarchy{Side: 8, From: 8}, units)
	r.sendEach(r.nw.grid.At(28, 28), every(8)...)
	r.update()
	r.built(t)
	for _, tc := range []struct {
		name   string
		update func()
	}{
		{"the hierarchy kept", func() { r.ask(every(8)...); r.update() }},
		{"the board changed", func() { r.nw.board.Touch(0); r.ask(every(8)...); r.update() }},
	} {
		for range 4 {
			tc.update()
		}
		if allocs := testing.AllocsPerRun(10, tc.update); allocs != 0 {
			t.Errorf("%s: an Update allocates %v times; want none", tc.name, allocs)
		}
	}
	if r.coord.Climbs() == 0 {
		t.Error("no route climbed")
	}
}

// a target on an island no route reaches is given up after one search, not a search for every
// cell round it: the hierarchy of the unit's class says no route reaches any
func TestRouting_AnUnreachableTargetIsSettledWithoutAFlood(t *testing.T) {
	// water round an island east of its column and between two of its rows, the column on the
	// border of two clusters 4 cells a side or inside one of them
	moat := func(column uint32) func(testBoard, func(x, y uint32) cell.ID) {
		return func(b testBoard, at func(x, y uint32) cell.ID) {
			for y := range uint32(8) {
				for x := column; x < 16; x++ {
					if x == column || y <= 1 || y >= 6 {
						b.Set(at(x, y), water)
					}
				}
			}
		}
	}
	for _, column := range []struct {
		where string
		x     uint32
	}{{"on a border", 11}, {"inside a cluster", 10}} {
		for _, s := range []Spacing{CellSpacing, BodySpacing} {
			t.Run(s.String()+", the moat "+column.where, func(t *testing.T) {
				g := grid.DefaultGrids{}.Square(16, 8, fieldCell)
				unit := fieldUnit{at: g.CellCenter(g.At(1, 4)), side: 8,
					order: &MoveOrder{Target: g.At(13, 3)}}
				// the searches a lost unit's ask takes over the world with the hierarchy given
				lost := func(h Hierarchy) (*fieldWorld, func() uint64) {
					fw := newFieldWorldWith(t, 16, 8, s, moat(column.x), []fieldUnit{unit},
						func(p *Plugin) { p.WithHierarchy(h) })
					coord := fw.nav.coordinator
					return fw, func() uint64 {
						before := coord.Searches()
						fw.each(func(_ uid.UID64, _ *world.Base, _ cell.ID, o *MoveOrder,
							_ *collision.Collider) {
							o.Path, o.Ask = Path{}, order.AskNearest
							o.Waited = spacing.TargetWaitTimeout
						})
						coord.Update(nil, time.Second/60)
						return coord.Searches() - before
					}
				}
				_, unkept := lost(Hierarchy{Off: true})
				flood := unkept()
				fw, kept := lost(Hierarchy{Side: 4, From: 4})
				// the first ask the Update that builds the hierarchy answers, the second a later
				for range 2 {
					if searched := kept(); searched != 1 || flood < 2 {
						t.Errorf("%d searches for a free cell round the island, want the first "+
							"alone, as against the %d of no hierarchy", searched, flood)
					}
					for range 8 {
						fw.nav.coordinator.Update(nil, time.Second/60)
					}
				}
				if _, o := fw.centre(0); o.Ask != order.NoPlace {
					t.Errorf("asked %v, want no place found", o.Ask)
				}
			})
		}
	}
}

// a class's hierarchy expected is built a few clusters a tick, the units asking nothing more
func TestRouting_TheHierarchyGrowsWithoutAsks(t *testing.T) {
	r := newClimbRig(t, nil, Hierarchy{Side: 8, From: 8}, climbers(westOf(1)))
	r.sendEach(r.nw.grid.At(28, 28), 0)
	r.update()
	searches, builds := r.coord.Searches(), r.coord.Builds()
	if builds == 0 {
		t.Fatal("no cluster built as the far route was asked")
	}
	// 4x4 clusters of 8 cells a side on one level
	for tick := 0; tick < 32 && r.coord.Builds() < 16; tick++ {
		before := r.coord.Builds()
		if r.nw.ecs.Tick(time.Second / 60); r.coord.Builds() == before {
			t.Fatalf("a tick built no cluster, %d of 16 built", before)
		}
	}
	if r.coord.Builds() != 16 || r.coord.Searches() != searches {
		t.Errorf("%d clusters built, %d searches more; want the 16, nothing asked",
			r.coord.Builds(), r.coord.Searches()-searches)
	}
}

// a party goes by its field: its class's hierarchy is never built for it
func TestRouting_APartyBuildsNoHierarchy(t *testing.T) {
	r := newClimbRig(t, nil, Hierarchy{Side: 8, From: 8}, climbers(westOf(8)))
	r.send(1, r.nw.grid.At(28, 28), every(8)...)
	r.update()
	r.routed(t, every(8)...)
	for range 4 {
		r.nw.ecs.Tick(time.Second / 60)
	}
	if c := r.coord; c.Fills() != 1 || c.Builds() != 0 || c.Climbs() != 0 {
		t.Errorf("%d fields filled, %d clusters built, %d climbs; want the party's field alone",
			c.Fills(), c.Builds(), c.Climbs())
	}
}

// the routes' board's hierarchy of a unit's class, built whole once its first far route is
// asked, which is searched meanwhile; the routes' rigs start from the north-west corner
func newBuiltLongRig(t *testing.T, pool *parallel.Pool, h Hierarchy, units []climber,
	keep ...cell.ID) *groupRig {
	t.Helper()
	warm := longGrid().At(2, routeSide-3)
	r := newLongRig(t, pool, h, units, append(keep, warm)...)
	r.sendEach(warm, every(len(units))...)
	r.update()
	r.built(t)
	return r
}

// a long route across the routes' board is climbed, not searched, once the hierarchy of the
// unit's class is built: 13x13 clusters of 16 cells a side, 4x4 over them
func TestRouting_ALongRouteIsClimbedNotSearched(t *testing.T) {
	g := longGrid()
	start, warm, goal := g.At(2, 2), g.At(2, routeSide-3), g.At(routeSide-3, routeSide-3)
	r := newLongRig(t, nil, Hierarchy{}, []climber{{at: start, side: 12}}, warm, goal)
	r.sendEach(warm, 0)
	r.update()
	if r.coord.Searches() != 1 || r.coord.Climbs() != 0 {
		t.Errorf("%d searches and %d climbs as the hierarchy was first asked of, want one search",
			r.coord.Searches(), r.coord.Climbs())
	}
	if built := r.built(t); built != 13*13+4*4 {
		t.Errorf("%d clusters built, want the %d of the board's two levels", built, 13*13+4*4)
	}
	searches, climbs := r.coord.Searches(), r.coord.Climbs()
	r.sendEach(goal, 0)
	r.update()
	r.chunked(t, 0)
	if r.coord.Searches() != searches || r.coord.Climbs() != climbs+1 {
		t.Errorf("%d searches and %d climbs more, want the route climbed",
			r.coord.Searches()-searches, r.coord.Climbs()-climbs)
	}
}

// a route of three chunks, asked again from each chunk's end, is three climbs and one route
func TestRouting_AChunkReaskClimbsFromTheChunksEnd(t *testing.T) {
	g := longGrid()
	start, goal := g.At(2, 100), g.At(172, 100)
	r := newBuiltLongRig(t, nil, Hierarchy{From: 32}, []climber{{at: start, side: 12}}, goal)
	searches, climbs := r.coord.Searches(), r.coord.Climbs()
	r.sendEach(goal, 0)
	var route []cell.ID
	asks := 0
	for asks < 8 {
		r.update()
		asks++
		r.chunked(t, 0)
		o, _ := r.order(r.nw.ids[0])
		route = append(route, o.Path.Steps[:o.Path.Length]...)
		end := o.Path.Steps[o.Path.Length-1]
		if end == goal {
			break
		}
		r.stand(0, end)
		r.ask(0)
	}
	if asks != 3 || r.coord.Climbs() != climbs+3 || r.coord.Searches() != searches {
		t.Errorf("%d asks, %d climbs and %d searches for a route of %d steps; want three asks, "+
			"each climbed", asks, r.coord.Climbs()-climbs, r.coord.Searches()-searches,
			len(route))
	}
	for k, prev := range append([]cell.ID{start}, route[:len(route)-1]...) {
		if _, ok := grid.Way(g, prev, route[k]); !ok {
			t.Fatalf("step %d of the route, %v to %v, is no step", k, prev, route[k])
		}
	}
}

// the same far question asked again is answered as before, neither climbed nor searched
func TestRouting_TheSameLongQuestionIsAnsweredUnrun(t *testing.T) {
	goal := longGrid().At(routeSide-3, routeSide-3)
	r := newBuiltLongRig(t, nil, Hierarchy{}, []climber{{at: longGrid().At(2, 2), side: 12}},
		goal)
	r.sendEach(goal, 0)
	r.update()
	first, _ := r.order(r.nw.ids[0])
	climbed := first.Path
	searches, climbs := r.coord.Searches(), r.coord.Climbs()
	r.ask(0)
	r.update()
	again, _ := r.order(r.nw.ids[0])
	if r.coord.Climbs() != climbs || r.coord.Searches() != searches || again.Path != climbed {
		t.Errorf("asked again: %d climbs and %d searches more, the route %v; want %v unrun",
			r.coord.Climbs()-climbs, r.coord.Searches()-searches,
			again.Path.Steps[:again.Path.Length], climbed.Steps[:climbed.Length])
	}
}

// a cell changed inside a cluster, off every cheapest way between the cluster's entrances, has
// that cluster built again alone, the one over it left as it is, the next far route climbed
func TestRouting_AChangedCellRebuildsItsClusterAlone(t *testing.T) {
	g := longGrid()
	goal := g.At(routeSide-3, routeSide-3)
	r := newBuiltLongRig(t, nil, Hierarchy{}, []climber{{at: g.At(2, 2), side: 12}}, goal)
	builds := r.coord.Builds()
	r.toggled(g.At(56, 56))
	if r.update(); r.coord.Builds()-builds != 1 || r.coord.Building() {
		t.Errorf("%d clusters built again, building on %v; want the changed cell's alone",
			r.coord.Builds()-builds, r.coord.Building())
	}
	climbs := r.coord.Climbs()
	r.sendEach(goal, 0)
	if r.update(); r.coord.Climbs() != climbs+1 {
		t.Error("the next far route not climbed")
	}
}

// a cell changed the tick a far route is asked has its clusters staled and built before the
// route is climbed through them, not refused
func TestRouting_AClimbFollowsAPassageMoveTheSameTick(t *testing.T) {
	g := longGrid()
	goal := g.At(routeSide-3, routeSide-3)
	r := newBuiltLongRig(t, nil, Hierarchy{}, []climber{{at: g.At(2, 2), side: 12}}, goal)
	searches, climbs := r.coord.Searches(), r.coord.Climbs()
	r.toggled(g.At(56, 56))
	r.sendEach(goal, 0)
	r.update()
	r.chunked(t, 0)
	if r.coord.Climbs() != climbs+1 || r.coord.Searches() != searches {
		t.Errorf("%d climbs and %d searches more, want the route climbed",
			r.coord.Climbs()-climbs, r.coord.Searches()-searches)
	}
}

// long routes of units of two classes answered on a pool's goroutines are the routes one
// goroutine alone gives them
func TestRouting_LongRoutesOnAPoolAreTheRoutesOfOneGoroutine(t *testing.T) {
	g := longGrid()
	var units []climber
	var goals []cell.ID
	for i := range uint32(8) {
		units = append(units, climber{at: g.At(2, 6+20*i), side: 6 + 6*(i%2)})
		goals = append(goals, g.At(routeSide-3, routeSide-7-20*i))
	}
	run := func(pool *parallel.Pool) ([]Path, [3]uint64) {
		r := newBuiltLongRig(t, pool, Hierarchy{}, units, goals...)
		for u, goal := range goals {
			r.sendEach(goal, u)
		}
		r.update()
		r.chunked(t, every(len(units))...)
		return r.paths(), [3]uint64{r.coord.Searches(), r.coord.Climbs(), r.coord.Builds()}
	}
	one, oneCounts := run(nil)
	many, manyCounts := run(parallel.New(4))
	samePaths(t, many, one, "on the pool")
	if manyCounts != oneCounts || oneCounts[1] != uint64(len(units)) {
		t.Errorf("searches, climbs and builds %v on the pool, %v on one goroutine; want every "+
			"route climbed", manyCounts, oneCounts)
	}
	if !slices.ContainsFunc(one, func(p Path) bool { return p.Length == MaxPathLength }) {
		t.Error("no route a chunk long")
	}
}

// the hierarchy built by one run is kept in the cache's file and read by the next, which builds
// nothing and climbs the same routes; a file of another version is built anew
func TestRouting_TheHierarchyKeptInTheCacheIsLoadedByTheNextRun(t *testing.T) {
	cache := Cache{Dir: filepath.Join(t.TempDir(), "routes"), Keep: true, Version: "t"}
	g := longGrid()
	start, goal := g.At(2, 2), g.At(routeSide-3, routeSide-3)
	units := []climber{{at: start, side: 12}}
	warm := g.At(2, routeSide-3)
	r := newKeptLongRig(t, nil, Hierarchy{}, cache, units, goal, warm)
	r.sendEach(warm, 0)
	r.built(t)
	r.update()
	r.coord.Hierarchies().Flush()
	file := filepath.Join(cache.Dir, "hierarchy-1.bin")
	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("the hierarchy built is not kept: %v", err)
	}
	// the route to the goal, climbed now the hierarchy is built
	r.sendEach(goal, 0)
	r.update()
	path := r.paths()[0]
	if path.Length == 0 || r.coord.Climbs() != 1 {
		t.Fatal("no route climbed by the run that built")
	}

	again := newKeptLongRig(t, nil, Hierarchy{}, cache, units, goal, warm)
	again.sendEach(goal, 0)
	again.update()
	if c := again.nw.nav.Read(0).Counts(); c.Loaded != 1 || c.Builds != 0 || c.Climbs != 1 {
		t.Fatalf("the next run counts %+v; want the hierarchy loaded, nothing built, the route "+
			"climbed", c)
	}
	if got := again.paths()[0]; !slices.Equal(got.Steps[:got.Length], path.Steps[:path.Length]) {
		t.Errorf("the next run climbs %v, the run that built %v", got.Steps[:got.Length],
			path.Steps[:path.Length])
	}
	again.update()
	again.coord.Hierarchies().Flush()
	if after, err := os.Stat(file); err != nil || after.Size() != info.Size() {
		t.Errorf("the next run wrote the file anew (%v, %v), want it as it was", after, err)
	}

	other := Cache{Dir: cache.Dir, Keep: false, Version: "u"}
	fresh := newKeptLongRig(t, nil, Hierarchy{}, other, units, goal, warm)
	fresh.sendEach(goal, 0)
	fresh.update()
	if c := fresh.nw.nav.Read(0).Counts(); c.Loaded != 0 || c.Builds == 0 {
		t.Errorf("a run of another version counts %+v; want the hierarchy built anew", c)
	}
	fresh.built(t)
	fresh.update()
	fresh.coord.Hierarchies().Flush()
	if after, err := os.Stat(file); err != nil || after.Size() != info.Size() {
		t.Errorf("a run keeping nothing wrote the file (%v, %v), want it as it was", after, err)
	}
}

// a cluster built again after a change is dumped to the file in its turn, and the next run
// loads the base with the dumps; a file cut short is built anew
func TestRouting_TheHierarchyDumpsWhatItBuiltAgainAndLoadsNothingSpoilt(t *testing.T) {
	cache := Cache{Dir: t.TempDir(), Keep: true, Version: "t"}
	g := longGrid()
	start, goal := g.At(2, 2), g.At(routeSide-3, routeSide-3)
	units := []climber{{at: start, side: 12}}
	r := newKeptLongRig(t, nil, Hierarchy{}, cache, units, goal)
	r.sendEach(goal, 0)
	r.built(t)
	r.update()
	r.coord.Hierarchies().Flush()
	file := filepath.Join(cache.Dir, "hierarchy-1.bin")
	base, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	r.toggled(g.At(56, 56))
	r.update()
	r.coord.Hierarchies().DumpEvery(0)
	r.update()
	r.coord.Hierarchies().Flush()
	if after, err := os.Stat(file); err != nil || after.Size() <= base.Size() {
		t.Fatalf("after a cluster built again the file is %v (%v), want the dump appended to %d",
			after, err, base.Size())
	}
	r.sendEach(goal, 0)
	r.update()
	path := r.paths()[0]

	again := newKeptLongRig(t, nil, Hierarchy{}, cache, units, goal)
	again.toggled(g.At(56, 56))
	again.sendEach(goal, 0)
	again.update()
	if c := again.nw.nav.Read(0).Counts(); c.Loaded != 1 || c.Builds != 0 {
		t.Fatalf("the next run counts %+v; want the base and the dump loaded, nothing built", c)
	}
	if got := again.paths()[0]; !slices.Equal(got.Steps[:got.Length], path.Steps[:path.Length]) {
		t.Errorf("the next run climbs %v, the run that dumped %v", got.Steps[:got.Length],
			path.Steps[:path.Length])
	}
	// a run over the board as laid finds the dumped cluster's cells otherwise and builds it again
	original := newKeptLongRig(t, nil, Hierarchy{Side: 16}, Cache{Dir: cache.Dir, Version: "t"},
		units, goal)
	original.sendEach(goal, 0)
	original.update()
	if c := original.nw.nav.Read(0).Counts(); c.Loaded != 1 || c.Builds == 0 || c.Builds > 20 {
		t.Errorf("a run over the board as laid counts %+v; want the file loaded and the "+
			"changed cell's clusters alone built again", c)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data[:len(data)*2/3], 0o644); err != nil {
		t.Fatal(err)
	}
	cut := newKeptLongRig(t, nil, Hierarchy{}, cache, units, goal)
	cut.sendEach(goal, 0)
	cut.update()
	if c := cut.nw.nav.Read(0).Counts(); c.Loaded != 0 || c.Builds == 0 {
		t.Errorf("a run over a file cut short counts %+v; want the hierarchy built anew", c)
	}
}

func TestBoard_FingerprintsWhatTheStepsReadOfACell(t *testing.T) {
	g := longGrid()
	units := []climber{{at: g.At(2, 2), side: 12}}
	a := newLongRig(t, nil, Hierarchy{}, units)
	b := newLongRig(t, nil, Hierarchy{}, units)
	c, other := g.At(56, 56), g.At(57, 56)
	if a.nw.board.CellFingerprint(c) != b.nw.board.CellFingerprint(c) {
		t.Error("two boards laid alike have other fingerprints of a cell")
	}
	was, beside := a.nw.board.CellFingerprint(c), a.nw.board.CellFingerprint(other)
	a.toggled(c)
	if a.nw.board.CellFingerprint(c) == was || a.nw.board.CellFingerprint(other) != beside {
		t.Error("a cell changed, its fingerprint is as it was or its neighbour's moved")
	}
}
