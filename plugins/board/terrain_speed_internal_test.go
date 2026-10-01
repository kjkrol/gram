package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
)

// steepMap is a Map whose every slope takes twice as long.
type steepMap struct{ Map }

func (steepMap) Slope(geom.Vec, geom.Vec, cell.Domain) float64 { return 2 }

// uphillMap climbs to the east: twice as long that way, half as long the other.
type uphillMap struct{ Map }

func (uphillMap) Slope(_ geom.Vec, dir geom.Vec, _ cell.Domain) float64 {
	if dir.X > 0 {
		return 2
	}
	return 0.5
}

// Backing away, facing up the slope, an entity goes down it: the slope is taken the way it goes.
func TestTerrainSpeed_TakesTheSlopeTheWayABackingEntityGoes(t *testing.T) {
	grid := DefaultGrids{}.Square(1, 1, 10)
	terrain := NewTerrainMap()
	c, _ := grid.CellIndex(0, 0)
	terrain.Set(c, cell.Kind{Cost: 1, Allows: cell.Land})
	brd := NewBoard(grid, terrain)
	brd.mapping = uphillMap{brd.Map()}
	got := speeds(t, brd, func(si *goke.SysInit, base *goke.Comp[world.Base]) {
		var mover goke.Comp[Mover]
		f := si.NewFactory(base, &mover)
		f.Create(1)
		for f.Next() {
			base.Slice(&f.Cursor)[0] = world.Base{Pos: world.Position{AABB: CellAABB(grid, c, 4)}, Vel: world.Velocity{Dir: geom.NewVec(1, 0), Value: -1}}
			mover.Slice(&f.Cursor)[0] = Mover{Domain: cell.Land}
		}
	})
	if v := got[CellAABB(grid, c, 4).TopLeft.X]; v != -2 {
		t.Errorf("backing down the slope at 1: speed %v, want -2, quicker down", v)
	}
}

// The slope slows whoever moves over it, but not over a Graded kind.
func TestTerrainSpeed_SparesAGradedKindTheSlope(t *testing.T) {
	grid := DefaultGrids{}.Square(2, 1, 10)
	terrain := NewTerrainMap()
	at := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }
	terrain.Set(at(0), cell.Kind{Cost: 1, Allows: cell.Land})
	terrain.Set(at(1), cell.Kind{Cost: 1, Allows: cell.Land, Graded: true})
	brd := NewBoard(grid, terrain)
	brd.mapping = steepMap{brd.Map()}

	got := speeds(t, brd, func(si *goke.SysInit, base *goke.Comp[world.Base]) {
		var mover goke.Comp[Mover]
		f := si.NewFactory(base, &mover)
		f.Create(2)
		x := uint32(0)
		for f.Next() {
			for i := range f.Cursor.IDs {
				base.Slice(&f.Cursor)[i] = world.Base{Pos: world.Position{AABB: CellAABB(grid, at(x), 4)}, Vel: world.Velocity{Dir: geom.NewVec(1, 0), Value: 1}}
				mover.Slice(&f.Cursor)[i] = Mover{Domain: cell.Land}
				x++
			}
		}
	})
	for x, want := range map[uint32]float64{0: 0.5, 1: 1} {
		x0 := CellAABB(grid, at(x), 4).TopLeft.X
		if got[x0] != want {
			t.Errorf("cell %d: speed %v, want %v", x, got[x0], want)
		}
	}
}

// speeds runs terrainSpeed over entities placed by place, all at speed 1, and reports each one's
// speed after, keyed by its box's left edge.
func speeds(t *testing.T, brd *Board, place func(si *goke.SysInit, base *goke.Comp[world.Base])) map[float64]float64 {
	t.Helper()
	host := &host.EachHost[world.Moving]{}
	if err := host.Add(terrainSpeed(brd)); err != nil {
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
	grid := DefaultGrids{}.Square(3, 1, 10)
	terrain := NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	at := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }
	terrain.Set(at(0), cell.Kind{Cost: 2, Allows: cell.Land})   // slow
	terrain.Set(at(1), cell.Kind{Cost: 0.5, Allows: cell.Land}) // a boost, if a game wants one
	terrain.Set(at(2), cell.Kind{Cost: 0, Allows: cell.Land})   // no cost: no effect

	got := speeds(t, NewBoard(grid, terrain), func(si *goke.SysInit, base *goke.Comp[world.Base]) {
		var mover goke.Comp[Mover]
		f := si.NewFactory(base, &mover)
		f.Create(3)
		x := uint32(0)
		for f.Next() {
			for i := range f.Cursor.IDs {
				base.Slice(&f.Cursor)[i] = world.Base{Pos: world.Position{AABB: CellAABB(grid, at(x), 4)}, Vel: world.Velocity{Value: 1}}
				mover.Slice(&f.Cursor)[i] = Mover{Domain: cell.Land}
				x++
			}
		}
	})
	for x, want := range map[uint32]float64{0: 0.5, 1: 2, 2: 1} {
		x0 := CellAABB(grid, at(x), 4).TopLeft.X
		if got[x0] != want {
			t.Errorf("cell %d: speed %v, want %v", x, got[x0], want)
		}
	}
}

func TestTerrainSpeed_ChargesTheEntitysOwnDomainAndSparesTheMoverless(t *testing.T) {
	const frost = cell.Domain(1 << 3)
	grid := DefaultGrids{}.Square(1, 1, 10)
	terrain := NewTerrainMap()
	c, _ := grid.CellIndex(0, 0)
	terrain.Set(c, cell.Kind{Name: cell.Named("snow"), Cost: 4, Allows: cell.Land | frost}.Costing(frost, 0.5))

	// Three entities on the snow, told apart by a one-unit offset: on foot, frost-born, no Mover.
	got := speeds(t, NewBoard(grid, terrain), func(si *goke.SysInit, base *goke.Comp[world.Base]) {
		var mover goke.Comp[Mover]
		box := CellAABB(grid, c, 4)
		f := si.NewFactory(base, &mover)
		f.Create(2)
		domains := []cell.Domain{cell.Land, cell.Land | frost}
		for f.Next() {
			for i := range f.Cursor.IDs {
				b := box
				b.TopLeft.X += float64(2 - len(domains))
				base.Slice(&f.Cursor)[i] = world.Base{Pos: world.Position{AABB: b}, Vel: world.Velocity{Value: 1}}
				mover.Slice(&f.Cursor)[i] = Mover{Domain: domains[0]}
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
	x0 := CellAABB(grid, c, 4).TopLeft.X
	if got[x0] != 0.25 || got[x0+1] != 2 || got[x0+2] != 1 {
		t.Errorf("speeds land=%v frost=%v moverless=%v, want 0.25, 2 and 1 untouched", got[x0], got[x0+1], got[x0+2])
	}
}
