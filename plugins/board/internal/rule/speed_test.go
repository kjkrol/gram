package rule

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
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
func TestTerrainSpeed_TakesTheSlopeTheWayABackingEntityGoes(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(1, 1, 10)
	cells := terrain.New(grid)
	c, _ := grid.CellIndex(0, 0)
	cells.Set(c, cell.Kind{Cost: 1, Allows: cell.Land})
	got := speeds(t, grid, cells, uphill, func(si *goke.SysInit, base *goke.Comp[world.Base]) {
		var mover goke.Comp[unit.Mover]
		f := si.NewFactory(base, &mover)
		f.Create(1)
		for f.Next() {
			base.Slice(&f.Cursor)[0] = world.Base{Pos: world.Position{AABB: cellBox(grid, c, 4)}, Vel: world.Velocity{Dir: geom.NewVec(1, 0), Value: -1}}
			mover.Slice(&f.Cursor)[0] = unit.Mover{Domain: cell.Land}
		}
	})
	if v := got[cellBox(grid, c, 4).TopLeft.X]; v != -2 {
		t.Errorf("backing down the slope at 1: speed %v, want -2, quicker down", v)
	}
}

// The slope slows whoever moves over it, but not over a Graded kind.
func TestTerrainSpeed_SparesAGradedKindTheSlope(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(2, 1, 10)
	cells := terrain.New(grid)
	at := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }
	cells.Set(at(0), cell.Kind{Cost: 1, Allows: cell.Land})
	cells.Set(at(1), cell.Kind{Cost: 1, Allows: cell.Land, Graded: true})

	got := speeds(t, grid, cells, steep, func(si *goke.SysInit, base *goke.Comp[world.Base]) {
		var mover goke.Comp[unit.Mover]
		f := si.NewFactory(base, &mover)
		f.Create(2)
		x := uint32(0)
		for f.Next() {
			for i := range f.Cursor.IDs {
				base.Slice(&f.Cursor)[i] = world.Base{Pos: world.Position{AABB: cellBox(grid, at(x), 4)}, Vel: world.Velocity{Dir: geom.NewVec(1, 0), Value: 1}}
				mover.Slice(&f.Cursor)[i] = unit.Mover{Domain: cell.Land}
				x++
			}
		}
	})
	for x, want := range map[uint32]float64{0: 0.5, 1: 1} {
		x0 := cellBox(grid, at(x), 4).TopLeft.X
		if got[x0] != want {
			t.Errorf("cell %d: speed %v, want %v", x, got[x0], want)
		}
	}
}

// speeds runs TerrainSpeed over entities placed by place, all at speed 1, and reports each one's
// speed after, keyed by its box's left edge.
func speeds(t *testing.T, g grid.Grid, cells *terrain.Cells, slope func(p, dir geom.Vec, d cell.Domain) float64, place func(si *goke.SysInit, base *goke.Comp[world.Base])) map[float64]float64 {
	t.Helper()
	host := &host.EachHost[world.Moving]{}
	if err := host.Add(TerrainSpeed(g, cells, slope)); err != nil {
		t.Fatal(err)
	}
	got := map[float64]float64{}
	var base goke.Comp[world.Base]
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		qb := si.NewQueryBuilder(&base)
		host.Bind(qb)
		q := qb.Build()
		place(si, &base)
		for q.All(); q.Next(); {
			cur := q.Cursor()
			bases := base.Slice(cur)
			host.Run(plugin.Tick{}, cur, func(i int) world.Moving { return world.Moving{ID: cur.IDs[i], Base: &bases[i]} })
			for i := range cur.IDs {
				got[bases[i].Pos.TopLeft.X] = bases[i].Vel.Value
			}
		}
	}})
	return got
}

func TestTerrainSpeed_ScalesByOneOverCost(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(3, 1, 10)
	cells := terrain.New(grid)
	cells.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	at := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }
	cells.Set(at(0), cell.Kind{Cost: 2, Allows: cell.Land})   // slow
	cells.Set(at(1), cell.Kind{Cost: 0.5, Allows: cell.Land}) // a boost, if a game wants one
	cells.Set(at(2), cell.Kind{Cost: 0, Allows: cell.Land})   // no cost: no effect

	got := speeds(t, grid, cells, flat, func(si *goke.SysInit, base *goke.Comp[world.Base]) {
		var mover goke.Comp[unit.Mover]
		f := si.NewFactory(base, &mover)
		f.Create(3)
		x := uint32(0)
		for f.Next() {
			for i := range f.Cursor.IDs {
				base.Slice(&f.Cursor)[i] = world.Base{Pos: world.Position{AABB: cellBox(grid, at(x), 4)}, Vel: world.Velocity{Value: 1}}
				mover.Slice(&f.Cursor)[i] = unit.Mover{Domain: cell.Land}
				x++
			}
		}
	})
	for x, want := range map[uint32]float64{0: 0.5, 1: 2, 2: 1} {
		x0 := cellBox(grid, at(x), 4).TopLeft.X
		if got[x0] != want {
			t.Errorf("cell %d: speed %v, want %v", x, got[x0], want)
		}
	}
}

func TestTerrainSpeed_ChargesTheEntitysOwnDomainAndSparesTheMoverless(t *testing.T) {
	const frost = cell.Domain(1 << 3)
	grid := grid.DefaultGrids{}.Square(1, 1, 10)
	cells := terrain.New(grid)
	c, _ := grid.CellIndex(0, 0)
	cells.Set(c, cell.Kind{Name: cell.Named("snow"), Cost: 4, Allows: cell.Land | frost}.Costing(frost, 0.5))

	// Three entities on the snow, told apart by a one-unit offset: on foot, frost-born, no Mover.
	got := speeds(t, grid, cells, flat, func(si *goke.SysInit, base *goke.Comp[world.Base]) {
		var mover goke.Comp[unit.Mover]
		box := cellBox(grid, c, 4)
		f := si.NewFactory(base, &mover)
		f.Create(2)
		domains := []cell.Domain{cell.Land, cell.Land | frost}
		for f.Next() {
			for i := range f.Cursor.IDs {
				b := box
				b.TopLeft.X += float64(2 - len(domains))
				base.Slice(&f.Cursor)[i] = world.Base{Pos: world.Position{AABB: b}, Vel: world.Velocity{Value: 1}}
				mover.Slice(&f.Cursor)[i] = unit.Mover{Domain: domains[0]}
				domains = domains[1:]
			}
		}
		g := si.NewFactory(base)
		g.Create(1)
		for g.Next() {
			b := box
			b.TopLeft.X += 2
			base.Slice(&g.Cursor)[0] = world.Base{Pos: world.Position{AABB: b}, Vel: world.Velocity{Value: 1}}
		}
	})
	x0 := cellBox(grid, c, 4).TopLeft.X
	if got[x0] != 0.25 || got[x0+1] != 2 || got[x0+2] != 1 {
		t.Errorf("speeds land=%v frost=%v moverless=%v, want 0.25, 2 and 1 untouched", got[x0], got[x0+1], got[x0+2])
	}
}

// cellBox is the size x size box centred on cell c.
func cellBox(g grid.Grid, c cell.ID, size uint32) plane.AABB {
	at := g.CellCenter(c)
	half := float64(size) / 2
	return plane.NewAABB(geom.NewVec(at.X-half, at.Y-half), float64(size), float64(size))
}
