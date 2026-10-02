package relief_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

func TestAltitude_IsTheGroundUnderTheUnitPlusItsLift(t *testing.T) {
	var walker, hawk kind.Of[topotest.Recruit]
	qw := topotest.NewQuasiWorld(t, false, func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
		walker = units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
		hawk = units.Define("hawk", unit.Mover{Domain: cell.Air, Lift: 40}, steering.Steering{MaxSpeed: 10})
		onHill, _ := grid.CellIndex(2, 1)
		onGrass, _ := grid.CellIndex(0, 3)
		return []kind.Entry{walker.Entry(topotest.Recruit{Start: onHill}), hawk.Entry(topotest.Recruit{Start: onHill}), walker.Entry(topotest.Recruit{Start: onGrass})}
	})
	qw.ECS.Tick(time.Second / 60)

	zs := qw.Zs()
	if got := zs[walker.ID()]; len(got) != 2 || got[0].Height != 2 || got[1].Height != 2 {
		t.Fatalf("walkers' Z = %v, want two, each 2 tall", got)
	}
	alts := map[float64]bool{}
	for _, z := range zs[walker.ID()] {
		alts[z.Altitude] = true
	}
	onHill, _ := qw.Grid.CellIndex(2, 1)
	hillGround := qw.Topo.Relief().At(qw.Grid.CellCenter(onHill))
	if hillGround <= 0 || !alts[hillGround] || !alts[0] {
		t.Errorf("walkers stand at %v, want one at the hill's ground %v and one at 0 on the grass", alts, hillGround)
	}
	if got := zs[hawk.ID()]; len(got) != 1 || got[0].Altitude != hillGround+40 || got[0].Height != 2 {
		t.Errorf("hawk's Z = %v, want altitude %v (the hill plus its lift) and height 2", got, hillGround+40)
	}
	if qw.Board.Heights() != qw.Topo.Relief() {
		t.Error("the board's heights are not the topography's relief")
	}
}

// A flyer flown by hand holds its height over sea level as the ground rises and falls under it,
// climbs along the way it is steered as far as it went, keeps its Clearance over the ground and
// stays under its Ceiling; let go, it keeps its height over the ground again.
func TestAltitude_AFlyerFlownByHandHoldsItsHeightOverSeaLevel(t *testing.T) {
	qw := topotest.NewQuasiWorld(t, false, func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
		hawk := units.Define("hawk", unit.Mover{Domain: cell.Air, Lift: 40}, steering.Steering{MaxSpeed: 10}, comp.Const(steering.Driven{}))
		onGrass, _ := grid.CellIndex(0, 3)
		return []kind.Entry{hawk.Entry(topotest.Recruit{Start: onGrass})}
	})
	onHill, _ := qw.Grid.CellIndex(2, 1)
	onGrass, _ := qw.Grid.CellIndex(0, 3)
	put := func(c cell.ID) func(*world.Base) {
		return func(b *world.Base) {
			at := qw.Grid.CellCenter(c)
			b.Pos.AABB = plane.NewAABB(geom.NewVec(at.X-10, at.Y-10), 20, 20)
		}
	}
	step := func(fn func(b *world.Base, z *world.Z, m *unit.Mover, d *steering.Driven, st *steering.Course)) (alt, lift float64) {
		flown(qw, fn)
		qw.ECS.Tick(time.Second / 60)
		flown(qw, func(_ *world.Base, z *world.Z, m *unit.Mover, _ *steering.Driven, _ *steering.Course) {
			alt, lift = z.Altitude, m.Lift
		})
		return alt, lift
	}
	hill := qw.Topo.Relief().At(qw.Grid.CellCenter(onHill))
	if alt, _ := step(func(*world.Base, *world.Z, *unit.Mover, *steering.Driven, *steering.Course) {}); alt != 40 {
		t.Fatalf("steered from nowhere it flies at %v, want its lift 40 over the grass", alt)
	}
	if alt, lift := step(func(b *world.Base, _ *world.Z, _ *unit.Mover, d *steering.Driven, _ *steering.Course) {
		*d = steering.Driven{Flown: true}
		put(onHill)(b)
	}); alt != 40 || lift != 40-hill {
		t.Errorf("flown over the hill %v high: at %v, lift %v; want at 40, lift %v", hill, alt, lift, 40-hill)
	}
	if alt, _ := step(func(b *world.Base, _ *world.Z, _ *unit.Mover, _ *steering.Driven, _ *steering.Course) {
		put(onGrass)(b)
	}); alt != 40 {
		t.Errorf("flown back over the grass: at %v, want at 40, not falling with the ground", alt)
	}
	alt, _ := step(func(_ *world.Base, _ *world.Z, _ *unit.Mover, d *steering.Driven, st *steering.Course) {
		d.Climb, st.Speed, st.WantSpeed = 0.6, 10, 10
	})
	if want := 40 + 10*0.75/60; math.Abs(alt-want) > 1e-6 {
		t.Errorf("flown at 10 a second at a rise of 0.6: at %v after a tick, want %v", alt, want)
	}
	if alt, _ := step(func(b *world.Base, _ *world.Z, m *unit.Mover, d *steering.Driven, st *steering.Course) {
		d.Climb, st.Speed, st.WantSpeed, m.Clearance = 0, 0, 0, 39
		put(onHill)(b)
	}); alt != hill+39 {
		t.Errorf("flown over the hill with a clearance of 39: at %v, want %v, the clearance over the ground", alt, hill+39)
	}
	if alt, _ := step(func(b *world.Base, _ *world.Z, m *unit.Mover, _ *steering.Driven, _ *steering.Course) {
		m.Clearance, m.Ceiling = 0, 41
		put(onGrass)(b)
	}); alt != 41 {
		t.Errorf("flown under a ceiling of 41: at %v, want held down to it", alt)
	}
	if alt, lift := step(func(b *world.Base, _ *world.Z, m *unit.Mover, d *steering.Driven, _ *steering.Course) {
		*d, m.Ceiling = steering.Driven{}, 0
		put(onHill)(b)
	}); alt != hill+41 || lift != 41 {
		t.Errorf("let go over the hill: at %v, lift %v; want its lift 41 over the ground again", alt, lift)
	}
}

// A flyer climbs no higher than its Ceiling over sea level: over high ground its lift is held down
// so it does not, down to the ground where the ground stands higher still.
func TestAltitude_AFlyerKeepsUnderItsCeiling(t *testing.T) {
	var hawk, low kind.Of[topotest.Recruit]
	qw := topotest.NewQuasiWorld(t, false, func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
		hawk = units.Define("hawk", unit.Mover{Domain: cell.Air, Lift: 40, Ceiling: 41}, steering.Steering{MaxSpeed: 10})
		low = units.Define("low", unit.Mover{Domain: cell.Air, Lift: 40, Ceiling: 1}, steering.Steering{MaxSpeed: 10})
		onHill, _ := grid.CellIndex(2, 1)
		onGrass, _ := grid.CellIndex(0, 3)
		return []kind.Entry{hawk.Entry(topotest.Recruit{Start: onHill}), hawk.Entry(topotest.Recruit{Start: onGrass}), low.Entry(topotest.Recruit{Start: onHill})}
	})
	qw.ECS.Tick(time.Second / 60)
	onHill, _ := qw.Grid.CellIndex(2, 1)
	hill := qw.Topo.Relief().At(qw.Grid.CellCenter(onHill))
	if hill <= 1 || hill+40 <= 41 {
		t.Fatalf("the hill stands %v high: too low to test the ceiling on", hill)
	}
	zs := qw.Zs()
	alts := map[float64]bool{}
	for _, z := range zs[hawk.ID()] {
		alts[z.Altitude] = true
	}
	if len(alts) != 2 || !alts[40] || !alts[41] {
		t.Errorf("hawks fly at %v, want one at its lift 40 over the grass and one held to the ceiling 41 over the hill", alts)
	}
	for _, z := range zs[low.ID()] {
		if z.Altitude != hill {
			t.Errorf("a flyer whose ceiling lies under the hill flies at %v, want on the hill %v", z.Altitude, hill)
		}
	}
}

// flown hands fn the one entity steered by hand, between two ticks.
func flown(qw *topotest.QuasiWorld, fn func(b *world.Base, z *world.Z, m *unit.Mover, d *steering.Driven, st *steering.Course)) {
	var base goke.Comp[world.Base]
	var z goke.Comp[world.Z]
	var mover goke.Comp[unit.Mover]
	var driven goke.Comp[steering.Driven]
	var steer goke.Comp[steering.Course]
	var q *goke.Query
	qw.ECS.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base, &z, &mover, &driven, &steer).Build() }})
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i := range cur.IDs {
			fn(&base.Slice(cur)[i], &z.Slice(cur)[i], &mover.Slice(cur)[i], &driven.Slice(cur)[i], &steer.Slice(cur)[i])
		}
	}
}
