package navigation

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// the cameras the world lives round, none at first: unobserved, every sector is ticked at the
// ladder's Unobserved, L4
type observed struct {
	w, h uint32
	cams []camera.Camera
}

var _ sector.Observers = (*observed)(nil)

func (o *observed) Observers(each func(cam camera.Camera)) {
	for _, cam := range o.cams {
		each(cam)
	}
}

// look has a camera look at (x, y) a pixel a world unit, zoomed out by out: under abstractLadder
// every sector L2 at 1, L3 at 2.2
func (o *observed) look(x, y, out float32) {
	// its picture shows every sector: the levels go by the scale and the distance alone
	cam := cameratest.Showing(cameratest.NewFromSpace(o.w, o.h, 0))
	cam.SetViewport(2, 2)
	if out > 1 {
		cam.ZoomOut(out, 0, 0)
	}
	cam.CenterOn(x, y, 0)
	o.cams = append(o.cams, cam)
}

// L2 from half a pixel a world unit, L3 from 0.4, L4 from 0.2, seeing without end
var abstractLadder = lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.4, 0.2}}

// abstractRig is a board of cols by rows cells of legCellSize cut into sectors of 16 cells a side
// (two of the space's buckets of 256), observed as seen says, its units as given
type abstractRig struct {
	nw     *navWorld
	g      grid.Grid
	seen   *observed
	q      *goke.Query
	base   goke.Comp[world.Base]
	at     goke.Comp[unit.At]
	orders goke.OptComp[MoveOrder]
	mark   goke.OptComp[sector.Abstract]
}

func newAbstractRig(t *testing.T, cols, rows uint32, units []navUnit,
	lay func(r *abstractRig)) *abstractRig {
	t.Helper()
	return newAbstractRigOn(t, nil, cols, rows, units, lay)
}

// newAbstractRigOn is newAbstractRig, the plugins working on pool, after set up last
func newAbstractRigOn(t *testing.T, pool *parallel.Pool, cols, rows uint32, units []navUnit,
	lay func(r *abstractRig), after ...goke.System) *abstractRig {
	t.Helper()
	r := &abstractRig{g: grid.DefaultGrids{}.Square(cols, rows, legCellSize),
		seen: &observed{w: cols * legCellSize, h: rows * legCellSize}}
	r.nw = newNavWorldIn(t, pool, cols, rows, legCellSize, units, world.SectorsCfg{Buckets: 2},
		abstractLadder, func(w *world.Plugin, _ *board.Plugin) { w.Write().Observe(r.seen) },
		append([]goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
			r.q = si.NewQueryBuilder(&r.base, &r.at).Optional(&r.orders, &r.mark).Build()
		}}}, after...)...)
	if lay != nil {
		lay(r)
	}
	return r
}

// a walker from (x, y) to (tx, ty), going speed cells a second at once and turning at once
func (r *abstractRig) walker(x, y, tx, ty uint32, speed float32) navUnit {
	start := r.g.At(x, y)
	profile := steering.Steering{MaxSpeed: speed * float32(legCellSize)}
	return navUnit{box: cellBox(r.g, start, legEntitySize), at: start, profile: &profile,
		order: &MoveOrder{Target: r.g.At(tx, ty)}}
}

// unit k as it stands: its order, nil with none, its box's middle, its cell, whether it is marked
func (r *abstractRig) unit(t *testing.T, k int) (*MoveOrder, float32, float32, cell.ID, bool) {
	t.Helper()
	if !r.q.Seek(r.nw.ids[k]) {
		t.Fatalf("unit %d is gone", k)
	}
	cur := r.q.Cursor()
	c := r.base.At(cur).Pos.Center()
	return r.orders.At(cur), c.X, c.Y, r.at.At(cur).Cell, r.mark.Present(cur)
}

func (r *abstractRig) tick() { r.nw.ecs.Tick(time.Second / 60) }

// the cells id holds
func (r *abstractRig) held(id uid.UID64) []cell.ID {
	var cells []cell.ID
	r.g.EachCell(func(c cell.ID) {
		r.nw.occupancy.Holders(c, func(e uid.UID64, _ cell.Domain, _ cell.Level) {
			if e == id {
				cells = append(cells, c)
			}
		})
	})
	return cells
}

// the walk's steps from p's From, each with what the planner prices it at
func (r *abstractRig) priced(t *testing.T, p Path) (steps []cell.ID, total float64) {
	t.Helper()
	prev := p.From
	for k, c := range p.Steps[:p.Length] {
		level := cell.OnGround
		if order.LevelAt(&p, uint16(k)) != cell.OnGround {
			level = cell.OnDeck
		}
		price, ok := r.nw.nav.finder.Price(prev, c, r.nw.board.Kind(c), level, cell.Land)
		if !ok {
			t.Fatalf("step %d of the route, %v to %v, refused", k, prev, c)
		}
		steps, total, prev = append(steps, c), total+price, c
	}
	return steps, total
}

func TestFar_WalksItsRouteOfCellsAtL4(t *testing.T) {
	const speed = 8 // cells a second
	r := &abstractRig{g: grid.DefaultGrids{}.Square(96, 16, legCellSize)}
	goal := r.g.At(60, 8)
	r = newAbstractRig(t, 96, 16, []navUnit{r.walker(2, 8, 60, 8, speed)}, func(r *abstractRig) {
		// a forest across the board, at half the pace
		for x := uint32(30); x < 38; x++ {
			for y := range uint32(16) {
				r.nw.board.Set(r.g.At(x, y), cell.Kind{Pace: 0.5, Allows: cell.Land})
			}
		}
	})
	read, id := r.nw.nav.Read(0), r.nw.ids[0]
	var given int
	var route Path
	for tick := 1; tick < 30 && given == 0; tick++ {
		r.tick()
		if o, _, _, _, _ := r.unit(t, 0); o.Path.Length > 0 {
			given, route = tick, o.Path
		}
	}
	if given == 0 {
		t.Fatal("no route given in half a second")
	}
	steps, total := r.priced(t, route)
	if steps[len(steps)-1] != goal {
		t.Fatalf("a route of %d steps to %v, want one to the goal %v", len(steps),
			steps[len(steps)-1], goal)
	}
	if c := read.Counts(); c.Abstracts != 1 || c.Climbs != 1 || c.Searches != 0 {
		t.Errorf("%d units far off, %d climbs, %d searches; want one far off, one climb, no "+
			"search", c.Abstracts, c.Climbs, c.Searches)
	}
	r.tick()
	o, x, _, _, marked := r.unit(t, 0)
	if o.Leg.Active || !marked {
		t.Errorf("a leg %v, marked %v; want none, marked", o.Leg.Active, marked)
	}
	if held := r.held(id); len(held) != 0 {
		t.Errorf("it holds %v, want nothing", held)
	}
	arrived := 0
	for tick := given + 2; tick < 1200 && arrived == 0; tick++ {
		r.tick()
		o, nx, ny, _, _ := r.unit(t, 0)
		if nx < x {
			t.Fatalf("at tick %d back from %v to %v, want it on along its route", tick, x, nx)
		}
		if x = nx; o == nil {
			arrived = tick
			break
		}
		if c, _ := r.g.CellAt(geom.NewVec(nx, ny)); !slices.Contains(steps, c) &&
			c != route.From {
			t.Fatalf("at tick %d on %v, off its route %v", tick, c, steps)
		}
		if len(r.held(id)) != 0 {
			t.Fatalf("at tick %d it holds %v on its way, want nothing", tick, r.held(id))
		}
	}
	// a step lasts its price at its top speed, a price being in steps between cells' middles
	want := float32(total) / speed * 60
	if took := float32(arrived - given); took < want-6 || took > want+6 {
		t.Errorf("there in %v ticks, want %v: the steps' prices %v at %d cells a second", took,
			want, total, speed)
	}
	r.tick()
	if _, _, _, at, marked := r.unit(t, 0); marked || at != goal || len(r.held(id)) != 1 {
		t.Errorf("there marked %v at %v holding %v; want it unmarked at its goal, holding it",
			marked, at, r.held(id))
	}
}

// a descent: the unit sent far, its route, the cell it was put on and came down onto, its route
// as it came down, what the routing had done as it was put there and after
type descent struct {
	route, after  Path
	on, down      cell.ID
	before, since Counts
	held          []cell.ID
}

// comeDown sends a unit from (2, 8) to (16, 8) round a wall but for its top row; walking its
// route far off it is put on the middle of its fourth step past its first ten, and the camera
// comes near. A unit stands on blocked from the start where it is not 0.
func comeDown(t *testing.T, blocked cell.ID) descent {
	t.Helper()
	r := &abstractRig{g: grid.DefaultGrids{}.Square(64, 16, legCellSize)}
	units := []navUnit{r.walker(2, 8, 16, 8, 4)}
	if blocked != 0 {
		units = append(units, navUnit{box: cellBox(r.g, blocked, legEntitySize), at: blocked})
	}
	r = newAbstractRig(t, 64, 16, units, func(r *abstractRig) {
		for x := uint32(6); x < 10; x++ {
			for y := uint32(1); y < 16; y++ {
				r.nw.board.Set(r.g.At(x, y), wall)
			}
		}
	})
	var d descent
	for range 30 {
		r.tick()
		if o, _, _, _, marked := r.unit(t, 0); marked && o.Path.Length > 10 {
			d.route = o.Path
			break
		}
	}
	if d.route.Length == 0 {
		t.Fatal("no route given far off")
	}
	o, _, _, _, _ := r.unit(t, 0)
	k := o.Path.Index + 3
	d.on = o.Path.Steps[k]
	at := r.g.CellCenter(d.on)
	half := float32(legEntitySize) / 2
	r.q.Seek(r.nw.ids[0])
	r.base.At(r.q.Cursor()).Pos.AABB = plane.NewAABB(geom.NewVec(at.X-half, at.Y-half), 2*half,
		2*half)
	d.before = r.nw.nav.Read(0).Counts()
	r.seen.look(0, 0, 1)
	for range 4 {
		r.tick()
		o, _, _, c, marked := r.unit(t, 0)
		if marked {
			continue
		}
		d.down, d.after, d.held = c, o.Path, r.held(r.nw.ids[0])
		break
	}
	if d.down == 0 {
		t.Fatal("never came down")
	}
	d.since = r.nw.nav.Read(0).Counts()
	return d
}

func TestFar_ComesDownOntoItsRouteAndWalksItOn(t *testing.T) {
	d := comeDown(t, 0)
	if d.since.Snaps != 0 {
		t.Errorf("%d snaps, want none", d.since.Snaps)
	}
	if d.down != d.on || !slices.Contains(d.held, d.on) {
		t.Errorf("down at %v holding %v, want it down where it was put, %v, holding it", d.down,
			d.held, d.on)
	}
	if d.after.Steps != d.route.Steps || d.after.Length != d.route.Length ||
		d.after.Index < 4 {
		t.Errorf("down with the route %v at %d, want its route %v walked on past where it was "+
			"put", d.after.Steps[:d.after.Length], d.after.Index, d.route.Steps[:d.route.Length])
	}
	if d.since.Climbs != d.before.Climbs || d.since.Searches != d.before.Searches {
		t.Errorf("coming down %d climbs and %d searches, want none", d.since.Climbs-d.before.Climbs,
			d.since.Searches-d.before.Searches)
	}
}

func TestFar_SnapsOffATakenCellAndAsksAnew(t *testing.T) {
	on := comeDown(t, 0).on
	d := comeDown(t, on)
	if d.since.Snaps != 1 || d.down == on {
		t.Errorf("with %v taken, down at %v after %d snaps; want it snapped off it", on, d.down,
			d.since.Snaps)
	}
	if !slices.Contains(d.held, d.down) {
		t.Errorf("snapped onto %v, it holds %v", d.down, d.held)
	}
	if d.after.Length != 0 && d.after.From != d.down {
		t.Errorf("snapped onto %v, its route %+v; want one asked anew from there", d.down, d.after)
	}
}

func TestFar_FinishesItsCellPathBeforeItGoesUp(t *testing.T) {
	r := &abstractRig{g: grid.DefaultGrids{}.Square(128, 16, legCellSize)}
	// far: a chunk of MaxPathLength steps at a time
	r = newAbstractRig(t, 128, 16, []navUnit{r.walker(2, 8, 120, 8, 8)}, nil)
	r.seen.look(float32(64*legCellSize), float32(8*legCellSize), 1)
	id, other := r.nw.ids[0], r.nw.ids[0]+1
	for range 30 {
		r.tick()
	}
	o, _, _, _, _ := r.unit(t, 0)
	if o.Path.Length == 0 {
		t.Fatalf("at L2 no path of cells: %+v", o)
	}
	first := o.Path
	r.seen.cams = nil
	var leg Leg
	walked := 0
	for range 1200 {
		r.tick()
		o, _, _, _, marked := r.unit(t, 0)
		if o == nil {
			t.Fatal("arrived on cells, never up in the far mode")
		}
		if marked {
			break
		}
		if o.Path.Index < o.Path.Length {
			walked++
		}
		if o.Leg.Active {
			leg = o.Leg
		}
		if len(r.held(id)) == 0 {
			t.Fatal("unmarked, it holds nothing")
		}
	}
	o, _, _, _, marked := r.unit(t, 0)
	if !marked || walked == 0 || o.Leg.Active || o.Path.From == first.From {
		t.Fatalf("marked %v after %d steps on its path, %+v; want its path walked, then marked "+
			"on the next, no leg", marked, walked, o)
	}
	if held := r.held(id); len(held) != 0 {
		t.Errorf("up in the far mode it holds %v, want nothing", held)
	}
	if !leg.Active {
		t.Fatal("no leg taken on its path at L4")
	}
	held, n := order.Held(leg)
	for _, h := range held[:n] {
		if !r.nw.occupancy.CanEnter(h.C, h.On, other, cell.Land) {
			t.Errorf("its last leg's %v still held", h.C)
		}
	}
	if c := r.nw.nav.Read(0).Counts(); c.Abstracts != 1 {
		t.Errorf("%d entries into the far mode, want one", c.Abstracts)
	}
}

func TestFar_WaitsForTheHierarchy(t *testing.T) {
	// a board too big to build in one step
	const side = 100
	g := grid.DefaultGrids{}.Square(side, side, 16)
	profile := steering.Steering{MaxSpeed: 64}
	start := g.At(2, 2)
	units := []navUnit{{box: cellBox(g, start, 12), at: start, profile: &profile,
		order: &MoveOrder{Target: g.At(side-3, side-3)}}}
	var orders goke.Comp[MoveOrder]
	var base goke.Comp[world.Base]
	var q *goke.Query
	nw := newNavWorldIn(t, nil, side, side, 16, units, world.SectorsCfg{Buckets: 2},
		abstractLadder, nil, goke.SystemFn{OnInit: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&orders, &base).Build()
		}})
	coord := nw.nav.module.navigationSystem.Routing.(*routing.Coordinator)
	state := func() (*MoveOrder, float32) {
		if !q.Seek(nw.ids[0]) {
			t.Fatal("the order is gone")
		}
		return orders.At(q.Cursor()), base.At(q.Cursor()).Pos.Center().X
	}
	_, x := state()
	waited := 0
	for tick := 0; tick < 300; tick++ {
		nw.ecs.Tick(time.Second / 60)
		o, nx := state()
		if o.Path.Length > 0 {
			break
		}
		if !coord.Building() && tick > 2 {
			t.Fatalf("at tick %d built, no route given: %+v", tick, o)
		}
		if o.Waited != 0 || o.Ask == order.AskNearest || nx != x {
			t.Fatalf("at tick %d waited %v, asking %v, from %v to %v; want it waiting unanswered "+
				"where it stands", tick, o.Waited, o.Ask, x, nx)
		}
		waited++
	}
	if o, _ := state(); o.Path.Length == 0 || waited < 3 {
		t.Errorf("after %d ticks waiting a route of %d steps, want it waiting a few, then given "+
			"one", waited, o.Path.Length)
	}
	if c := nw.nav.Read(0).Counts(); c.Searches != 0 || c.Climbs != 1 {
		t.Errorf("%d searches, %d climbs; want the route climbed once built, none searched",
			c.Searches, c.Climbs)
	}
}

func TestFar_MarchesAGroupAsUnits(t *testing.T) {
	r := &abstractRig{g: grid.DefaultGrids{}.Square(96, 16, legCellSize)}
	units := make([]navUnit, 6)
	for i := range units {
		units[i] = r.walker(uint32(2+i%3), uint32(7+i/3), 2, 7, 4)
		units[i].order, units[i].selected = nil, true
	}
	r = newAbstractRig(t, 96, 16, units, nil)
	r.tick()
	r.nw.nav.marches.Add(command.Nobody, March{Cell: r.g.At(80, 8), In: ColumnOf(1, true, 1)})
	for range 10 {
		r.tick()
	}
	for k := range units {
		o, _, _, _, marked := r.unit(t, k)
		if o == nil || o.Path.Length == 0 || o.Place != 0 || !marked {
			t.Errorf("unit %d: %+v, marked %v; want it on a route of its own, no place, marked",
				k, o, marked)
		}
	}
	if c := r.nw.nav.Read(0).Counts(); c.Fills != 0 || c.Walks != 0 {
		t.Errorf("%d fields filled, %d routes walked off them; want none", c.Fills, c.Walks)
	}
}

// units asking from one cell, two by two, on a pool's goroutines get the routes one goroutine
// alone gives them, and walk them alike
func TestFar_RoutesOnAPoolAreTheRoutesOfOneGoroutine(t *testing.T) {
	routes := func(pool *parallel.Pool) []Path {
		r := &abstractRig{g: grid.DefaultGrids{}.Square(96, 16, legCellSize)}
		var units []navUnit
		for k := range uint32(24) {
			units = append(units, r.walker(2+k/2%4, 1+k/8*5, 50+k/2%3*20, 2+k/2%14, 4))
		}
		r = newAbstractRigOn(t, pool, 96, 16, units, func(r *abstractRig) {
			for y := range uint32(16) {
				if y%5 != 2 {
					r.nw.board.Set(r.g.At(30, y), wall)
				}
			}
		})
		got := make([]Path, len(units))
		for range 30 {
			r.tick()
		}
		for k := range units {
			o, _, _, _, _ := r.unit(t, k)
			if o == nil || o.Path.Length == 0 {
				t.Fatalf("unit %d: %+v, want it on a route", k, o)
			}
			got[k] = o.Path
		}
		return got
	}
	alone, pooled := routes(nil), routes(parallel.New(4))
	for k := range alone {
		if alone[k] != pooled[k] {
			t.Errorf("unit %d: on a pool %+v, alone %+v", k, pooled[k], alone[k])
		}
	}
}
