package moments

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
)

func flat(cell.ID, cell.ID) float64  { return 0 }
func steep(cell.ID, cell.ID) float64 { return 0.3 }

// up to the east, down to the west
func uphill(from, to cell.ID) float64 {
	if to > from {
		return 0.3
	}
	return -0.3
}

// the share Tobler's hiking function leaves at slope on a kind of share 1
func hiking(slope float64) float32 {
	return float32(cell.DefaultHiking.Pace(cell.Step{Kind: cell.Kind{Pace: 1, Allows: cell.Land},
		Domain: cell.Land, Slope: slope}))
}

func near(got, want float32) bool { return math.Abs(float64(got-want)) < 1e-3 }

func TestPace_TakesTheSlopeTheWayABackingEntityGoes(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(3, 1, 10)
	cells := terrain.New(grid)
	cells.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	c := grid.At(1, 0)
	backing := walker{box: cellBox(grid, c, 4), vel: world.Velocity{Dir: geom.NewVec(1, 0),
		Value: -1}, domain: cell.Land}
	got := paces(t, grid, cells, uphill, backing)
	if p, want := got[cellBox(grid, c, 4).TopLeft.X], hiking(-0.3); !near(p, want) {
		t.Errorf("backing down the slope: pace %v, want %v, the descent's", p, want)
	}
}

func TestPace_SparesAGradedKindTheSlope(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(3, 1, 10)
	cells := terrain.New(grid)
	at := func(x uint32) cell.ID { c := grid.At(x, 0); return c }
	cells.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	cells.Set(at(1), cell.Kind{Pace: 1, Allows: cell.Land, Graded: true})

	var walkers []walker
	for x := range uint32(2) {
		walkers = append(walkers, walker{box: cellBox(grid, at(x), 4),
			vel: world.Velocity{Dir: geom.NewVec(1, 0), Value: 1}, domain: cell.Land})
	}
	got := paces(t, grid, cells, steep, walkers...)
	for x, want := range map[uint32]float32{0: hiking(0.3), 1: 1} {
		x0 := cellBox(grid, at(x), 4).TopLeft.X
		if !near(got[x0], want) {
			t.Errorf("cell %d: pace %v, want %v", x, got[x0], want)
		}
	}
}

func TestPace_IsTheKindsShare(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(3, 1, 10)
	cells := terrain.New(grid)
	at := func(x uint32) cell.ID { c := grid.At(x, 0); return c }
	cells.Set(at(0), cell.Kind{Pace: 0.5, Allows: cell.Land})
	cells.Set(at(1), cell.Kind{Pace: 1, Allows: cell.Land})
	cells.Set(at(2), cell.Kind{Pace: 0.25, Allows: cell.Land})

	var walkers []walker
	for x := range uint32(3) {
		walkers = append(walkers, walker{box: cellBox(grid, at(x), 4),
			vel: world.Velocity{Value: 1}, domain: cell.Land})
	}
	got := paces(t, grid, cells, flat, walkers...)
	for x, want := range map[uint32]float32{0: 0.5, 1: 1, 2: 0.25} {
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
	c := grid.At(0, 0)
	cells.Set(c, cell.Kind{Name: cell.Named("snow"), Pace: 0.25, Allows: cell.Land | frost}.
		Paced(frost, 1))

	// told apart by a one-unit offset
	box := cellBox(grid, c, 4)
	shifted := func(dx float32) plane.AABB { b := box; b.TopLeft.X += dx; return b }
	got := paces(t, grid, cells, flat,
		walker{box: shifted(0), vel: world.Velocity{Value: 1}, domain: cell.Land},
		walker{box: shifted(1), vel: world.Velocity{Value: 1}, domain: cell.Land | frost},
		walker{box: shifted(2), vel: world.Velocity{Value: 1}},
	)
	x0 := box.TopLeft.X
	if got[x0] != 0.25 || got[x0+1] != 1 || got[x0+2] != 1 {
		t.Errorf("paces land=%v frost=%v moverless=%v, want 0.25, 1 and 1 untouched",
			got[x0], got[x0+1], got[x0+2])
	}
}

// flat on the step onto to, a rise of 0.3 on every other
func flatOnto(to cell.ID) func(cell.ID, cell.ID) float64 {
	return func(_, t cell.ID) float64 {
		if t == to {
			return 0
		}
		return 0.3
	}
}

func TestPace_IsPricedByTheStepLedWhereItLiesAhead(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 3, 10)
	cells := terrain.New(grid)
	cells.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	c, east, west, far := grid.At(2, 1), grid.At(3, 1), grid.At(1, 1), grid.At(0, 1)
	eastward := world.Velocity{Dir: geom.NewVec(0.8, 0.6), Value: 1} // nearest the diagonal
	cases := []struct {
		name string
		next cell.ID
		vel  world.Velocity
		flat cell.ID
		want float32
	}{
		{"led east within a quarter turn", east, eastward, east, 1},
		{"led behind its back", west, eastward, west, hiking(0.3)},
		{"led into no neighbour", far, eastward, east, hiking(0.3)},
		{"led within its own cell, on the level", c, eastward, west, 1},
		{"led nowhere", 0, eastward, east, hiking(0.3)},
		{"backing west, led west", west, world.Velocity{Dir: geom.NewVec(1, 0), Value: -1}, west, 1},
	}
	for _, tc := range cases {
		w := walker{box: cellBox(grid, c, 4), vel: tc.vel, domain: cell.Land, next: tc.next}
		got := paces(t, grid, cells, flatOnto(tc.flat), w)
		if p := got[cellBox(grid, c, 4).TopLeft.X]; !near(p, tc.want) {
			t.Errorf("%s: pace %v, want %v", tc.name, p, tc.want)
		}
	}
}

// a mobility refusing every step onto a cell
type refusing map[cell.ID]bool

func (r refusing) Pace(s cell.Step) float64 {
	if r[s.To] {
		return 0
	}
	return s.Kind.PaceFor(s.Domain)
}

func TestPace_AZeroShareStopsTheUnit(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(2, 1, 10)
	cells := terrain.New(grid)
	cells.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	at := func(x uint32) cell.ID { c := grid.At(x, 0); return c }
	r := New(grid, cells, nil, flat, nil)
	r.Mobility = refusing{at(1): true}
	got := walk(t, r, 1,
		walker{box: cellBox(grid, at(0), 4), vel: world.Velocity{Value: 1}, domain: cell.Land},
		walker{box: cellBox(grid, at(1), 4), vel: world.Velocity{Value: 1}, domain: cell.Land})
	allowed := got[cellBox(grid, at(0), 4).TopLeft.X]
	refused := got[cellBox(grid, at(1), 4).TopLeft.X]
	if allowed != 1 || refused != 0 {
		t.Errorf("paces %v and %v, want 1 where the mobility allows and 0 where it refuses",
			allowed, refused)
	}
}

type walker struct {
	box    plane.AABB
	vel    world.Velocity
	domain cell.Domain
	next   cell.ID
}

func paces(t *testing.T, g grid.Grid, cells *terrain.Cells, rise func(from, to cell.ID) float64,
	walkers ...walker) map[float32]float32 {
	t.Helper()
	r := New(g, cells, nil, rise, nil)
	r.Mobility = cell.DefaultHiking
	return walk(t, r, 1, walkers...)
}

func walk(t *testing.T, r *Rules, steps int, walkers ...walker) map[float32]float32 {
	t.Helper()
	g, sys := r.grid, r.StandingSystem()
	got := map[float32]float32{}
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
				at.Slice(&f.Cursor)[0] = unit.At{Cell: c, Next: w.next}
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

func cellBox(g grid.Grid, c cell.ID, size uint32) plane.AABB {
	at := g.CellCenter(c)
	half := float32(size) / 2
	return plane.NewAABB(geom.NewVec(at.X-half, at.Y-half), float32(size), float32(size))
}
