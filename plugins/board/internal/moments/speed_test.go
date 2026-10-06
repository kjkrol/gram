package moments

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

// flat prices no slope; steep has every slope take twice as long.
func flat(geom.Vec, geom.Vec, cell.Domain) float64  { return 1 }
func steep(geom.Vec, geom.Vec, cell.Domain) float64 { return 2 }

// uphill climbs to the east: twice as long that way, half as long the other.
func uphill(_ geom.Vec, dir geom.Vec, _ cell.Domain) float64 {
	if dir.X > 0 {
		return 2
	}
	return 0.5
}

// Backing away, facing up the slope, an entity goes down it: the slope is taken the way it goes.
func TestPace_TakesTheSlopeTheWayABackingEntityGoes(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(1, 1, 10)
	cells := terrain.New(grid)
	c := grid.CellIndex(0, 0)
	cells.Set(c, cell.Kind{Cost: 1, Allows: cell.Land})
	got := paces(t, grid, cells, uphill, walker{box: cellBox(grid, c, 4), vel: world.Velocity{Dir: geom.NewVec(1, 0), Value: -1}, domain: cell.Land})
	if p := got[cellBox(grid, c, 4).TopLeft.X]; p != 2 {
		t.Errorf("backing down the slope: pace %v, want 2, quicker down", p)
	}
}

// The slope slows whoever moves over it, but not over a Graded kind.
func TestPace_SparesAGradedKindTheSlope(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(2, 1, 10)
	cells := terrain.New(grid)
	at := func(x uint32) cell.ID { c := grid.CellIndex(x, 0); return c }
	cells.Set(at(0), cell.Kind{Cost: 1, Allows: cell.Land})
	cells.Set(at(1), cell.Kind{Cost: 1, Allows: cell.Land, Graded: true})

	var walkers []walker
	for x := range uint32(2) {
		walkers = append(walkers, walker{box: cellBox(grid, at(x), 4), vel: world.Velocity{Dir: geom.NewVec(1, 0), Value: 1}, domain: cell.Land})
	}
	got := paces(t, grid, cells, steep, walkers...)
	for x, want := range map[uint32]float64{0: 0.5, 1: 1} {
		x0 := cellBox(grid, at(x), 4).TopLeft.X
		if got[x0] != want {
			t.Errorf("cell %d: pace %v, want %v", x, got[x0], want)
		}
	}
}

// walker is one entity of a paces run: its box, how it moves and its domain; none, no Mover.
type walker struct {
	box    plane.AABB
	vel    world.Velocity
	domain cell.Domain
}

// paces runs the board's standing pass once over walkers, each At the cell under it with a Pace
// of 1, and reports each one's Pace after, keyed by its box's left edge.
func paces(t *testing.T, g grid.Grid, cells *terrain.Cells, slope func(p, dir geom.Vec, d cell.Domain) float64, walkers ...walker) map[float64]float64 {
	t.Helper()
	return walk(t, New(g, cells, nil, slope), 1, walkers...)
}

// walk runs r's standing pass steps times over walkers, as paces does.
func walk(t *testing.T, r *Rules, steps int, walkers ...walker) map[float64]float64 {
	t.Helper()
	g, sys := r.grid, r.StandingSystem()
	got := map[float64]float64{}
	var (
		base  goke.Comp[world.Base]
		at    goke.Comp[unit.At]
		mover goke.Comp[unit.Mover]
		pace  goke.Comp[steering.Pace]
	)
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for _, w := range walkers {
			c, _ := g.CellAt(world.Position{AABB: w.box}.Center())
			f := si.NewFactory(&base, &at, &pace)
			if w.domain != 0 {
				f = si.NewFactory(&base, &at, &pace, &mover)
			}
			f.Create(1)
			for f.Next() {
				base.Slice(&f.Cursor)[0] = world.Base{Pos: world.Position{AABB: w.box}, Vel: w.vel}
				at.Slice(&f.Cursor)[0] = unit.At{Cell: c}
				pace.Slice(&f.Cursor)[0] = steering.Pace{Share: 1}
				if w.domain != 0 {
					mover.Slice(&f.Cursor)[0] = unit.Mover{Domain: w.domain}
				}
			}
		}
		sys.Init(si)
		for range steps {
			sys.Update(nil, time.Second/60)
		}
		q := si.NewQueryBuilder(&base, &pace).Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			bases, ps := base.Slice(cur), pace.Slice(cur)
			for i := range cur.IDs {
				got[bases[i].Pos.TopLeft.X] = ps[i].Share
			}
		}
	}})
	return got
}

func TestPace_IsOneOverCost(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(3, 1, 10)
	cells := terrain.New(grid)
	cells.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	at := func(x uint32) cell.ID { c := grid.CellIndex(x, 0); return c }
	cells.Set(at(0), cell.Kind{Cost: 2, Allows: cell.Land})   // slow
	cells.Set(at(1), cell.Kind{Cost: 0.5, Allows: cell.Land}) // a boost, if a game wants one
	cells.Set(at(2), cell.Kind{Cost: 0, Allows: cell.Land})   // no cost: no effect

	var walkers []walker
	for x := range uint32(3) {
		walkers = append(walkers, walker{box: cellBox(grid, at(x), 4), vel: world.Velocity{Value: 1}, domain: cell.Land})
	}
	got := paces(t, grid, cells, flat, walkers...)
	for x, want := range map[uint32]float64{0: 0.5, 1: 2, 2: 1} {
		x0 := cellBox(grid, at(x), 4).TopLeft.X
		if got[x0] != want {
			t.Errorf("cell %d: pace %v, want %v", x, got[x0], want)
		}
	}
}

func TestPace_ChargesTheEntitysOwnDomainAndSparesTheMoverless(t *testing.T) {
	const frost = cell.Domain(1 << 3)
	grid := grid.DefaultGrids{}.Square(1, 1, 10)
	cells := terrain.New(grid)
	c := grid.CellIndex(0, 0)
	cells.Set(c, cell.Kind{Name: cell.Named("snow"), Cost: 4, Allows: cell.Land | frost}.Costing(frost, 0.5))

	// Three entities on the snow, told apart by a one-unit offset: on foot, frost-born, no Mover.
	box := cellBox(grid, c, 4)
	shifted := func(dx float64) plane.AABB { b := box; b.TopLeft.X += dx; return b }
	got := paces(t, grid, cells, flat,
		walker{box: shifted(0), vel: world.Velocity{Value: 1}, domain: cell.Land},
		walker{box: shifted(1), vel: world.Velocity{Value: 1}, domain: cell.Land | frost},
		walker{box: shifted(2), vel: world.Velocity{Value: 1}},
	)
	x0 := box.TopLeft.X
	if got[x0] != 0.25 || got[x0+1] != 2 || got[x0+2] != 1 {
		t.Errorf("paces land=%v frost=%v moverless=%v, want 0.25, 2 and 1 untouched", got[x0], got[x0+1], got[x0+2])
	}
}

// cellBox is the size x size box centred on cell c.
func cellBox(g grid.Grid, c cell.ID, size uint32) plane.AABB {
	at := g.CellCenter(c)
	half := float64(size) / 2
	return plane.NewAABB(geom.NewVec(at.X-half, at.Y-half), float64(size), float64(size))
}
