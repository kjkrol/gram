package relief_test

import (
	"bytes"
	"log"
	"math"
	"strings"
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
		units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
		walker = units.Named("walker")
		units.Define("hawk", unit.Mover{Domain: cell.Air, Lift: 40}, steering.Steering{MaxSpeed: 10})
		hawk = units.Named("hawk")
		onHill := grid.At(2, 1)
		onGrass := grid.At(0, 3)
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
	onHill := qw.Grid.At(2, 1)
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

func TestAltitude_AFloorCarriesItsDomainsAtItsHeight(t *testing.T) {
	var walker, hawk, bridge kind.Of[topotest.Recruit]
	define := func(qw *topotest.QuasiWorld, units *board.Units[topotest.Recruit]) []kind.Entry {
		walks := steering.Steering{MaxSpeed: 10}
		units.Define("walker", unit.Mover{Domain: cell.Land}, walks)
		walker = units.Named("walker")
		units.Define("hawk", unit.Mover{Domain: cell.Air, Lift: 40}, walks)
		hawk = units.Named("hawk")
		shape := board.Shape{Size: 20, Height: 1}
		standing := board.NewEntities(qw.Board, shape, func(r topotest.Recruit) cell.ID {
			return r.Start
		})
		standing.Define("bridge", comp.Const(cell.Floor{Allows: cell.Land, Height: 30}))
		bridge = standing.Named("bridge")
		onHill := topotest.Recruit{Start: qw.Grid.At(2, 1)}
		return []kind.Entry{bridge.Entry(onHill), walker.Entry(onHill), hawk.Entry(onHill)}
	}
	qw := topotest.NewQuasiWorldWith(t, false, define)
	onHill := qw.Grid.At(2, 1)
	ground := qw.Topo.Relief().At(qw.Grid.CellCenter(onHill))
	deck := qw.Topo.Relief().Altitude(onHill) + 30
	qw.ECS.Tick(time.Second / 60)
	standAt(qw, cell.Land, deck)
	qw.ECS.Tick(time.Second / 60)
	zs := qw.Zs()
	if got := zs[walker.ID()]; len(got) != 1 || got[0].Altitude != deck {
		t.Errorf("the walker on the bridge: %v, want it at the deck %v", got, deck)
	}
	if got := zs[hawk.ID()]; len(got) != 1 || got[0].Altitude != ground+40 {
		t.Errorf("the hawk over the bridge: %v, want it 40 over the ground, %v", got, ground+40)
	}
	if got := zs[bridge.ID()]; len(got) != 1 || got[0].Altitude != deck || got[0].Height != 1 {
		t.Errorf("the bridge: %v, want it 1 tall on its deck at %v", got, deck)
	}
}

func TestAltitude_ADeckIsSteppedOntoWithinAStep(t *testing.T) {
	define := func(qw *topotest.QuasiWorld, units *board.Units[topotest.Recruit]) []kind.Entry {
		walks := steering.Steering{MaxSpeed: 10}
		units.Define("walker", unit.Mover{Domain: cell.Land}, walks, comp.Const(steering.Driven{}))
		standing := board.NewEntities(qw.Board, board.Shape{Size: 20, Height: 1},
			func(r topotest.Recruit) cell.ID { return r.Start })
		standing.Define("bridge", comp.Const(cell.Floor{Allows: cell.Land, Height: 10}))
		under := topotest.Recruit{Start: qw.Grid.At(1, 2)}
		return []kind.Entry{
			standing.Named("bridge").Entry(under),
			units.Named("walker").Entry(under),
		}
	}
	qw := topotest.NewQuasiWorldWith(t, false, define)
	deck := qw.Topo.Relief().Altitude(qw.Grid.At(1, 2)) + 10
	step := func(alt float64) float64 {
		flown(qw, func(_ *world.Base, z *world.Z, _ *unit.Mover, _ *steering.Driven,
			_ *steering.Course) {
			z.Altitude = alt
		})
		qw.ECS.Tick(time.Second / 60)
		var got float64
		flown(qw, func(_ *world.Base, z *world.Z, _ *unit.Mover, _ *steering.Driven,
			_ *steering.Course) {
			got = z.Altitude
		})
		return got
	}
	reach := 2.0 / 4 // a quarter of the walker's Height
	if got := step(0); got == deck {
		t.Errorf("standing on the ground under the deck it went up onto it")
	}
	if got := step(deck - reach*0.8); got != deck {
		t.Errorf("a step under the deck: at %v, want on the deck at %v", got, deck)
	}
	if got := step(deck); got != deck {
		t.Errorf("on the deck: at %v, want still on it at %v", got, deck)
	}
	if got := step(deck - reach*1.5); got == deck {
		t.Errorf("more than a step under the deck it went up onto it")
	}
}

func TestStanding_UnderADeckOverWaterHasFallenIn(t *testing.T) {
	define := func(qw *topotest.QuasiWorld, units *board.Units[topotest.Recruit]) []kind.Entry {
		river := qw.Grid.At(1, 2)
		qw.Board.Board().Set(river, cell.Kind{Name: cell.Named("river"), Cost: 1,
			Allows: cell.Water})
		units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
		standing := board.NewEntities(qw.Board, board.Shape{Size: 20, Height: 1},
			func(r topotest.Recruit) cell.ID { return r.Start })
		standing.Define("bridge", comp.Const(cell.Floor{Allows: cell.Land, Height: 10}))
		at := topotest.Recruit{Start: river}
		return []kind.Entry{standing.Named("bridge").Entry(at), units.Named("walker").Entry(at)}
	}
	qw := topotest.NewQuasiWorldWith(t, false, define)
	var out bytes.Buffer
	qw.Board.WithLog(log.New(&out, "", 0))
	deck := qw.Topo.Relief().Altitude(qw.Grid.At(1, 2)) + 10
	standAt(qw, cell.Land, deck)
	qw.ECS.Tick(time.Second / 60)
	qw.ECS.Tick(time.Second / 60)
	if out.Len() != 0 {
		t.Errorf("on the deck: %q, want no fall", out.String())
	}
	standAt(qw, cell.Land, 0)
	qw.ECS.Tick(time.Second / 60)
	if !strings.Contains(out.String(), "fell into the river") {
		t.Errorf("under the deck: %q, want it fallen into the river", out.String())
	}
}

func TestAltitude_AFlyerFlownByHandHoldsItsHeightOverSeaLevel(t *testing.T) {
	qw := topotest.NewQuasiWorld(t, false, func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
		units.Define("hawk", unit.Mover{Domain: cell.Air, Lift: 40}, steering.Steering{MaxSpeed: 10}, comp.Const(steering.Driven{}))
		hawk := units.Named("hawk")
		onGrass := grid.At(0, 3)
		return []kind.Entry{hawk.Entry(topotest.Recruit{Start: onGrass})}
	})
	onHill := qw.Grid.At(2, 1)
	onGrass := qw.Grid.At(0, 3)
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

func TestAltitude_AFlyerKeepsUnderItsCeiling(t *testing.T) {
	var hawk, low kind.Of[topotest.Recruit]
	qw := topotest.NewQuasiWorld(t, false, func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
		units.Define("hawk", unit.Mover{Domain: cell.Air, Lift: 40, Ceiling: 41}, steering.Steering{MaxSpeed: 10})
		hawk = units.Named("hawk")
		units.Define("low", unit.Mover{Domain: cell.Air, Lift: 40, Ceiling: 1}, steering.Steering{MaxSpeed: 10})
		low = units.Named("low")
		onHill := grid.At(2, 1)
		onGrass := grid.At(0, 3)
		return []kind.Entry{hawk.Entry(topotest.Recruit{Start: onHill}), hawk.Entry(topotest.Recruit{Start: onGrass}), low.Entry(topotest.Recruit{Start: onHill})}
	})
	qw.ECS.Tick(time.Second / 60)
	onHill := qw.Grid.At(2, 1)
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

func standAt(qw *topotest.QuasiWorld, d cell.Domain, alt float64) {
	var z goke.Comp[world.Z]
	var mover goke.Comp[unit.Mover]
	var q *goke.Query
	qw.ECS.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&z, &mover).Build()
	}})
	for q.All(); q.Next(); {
		zs, movers := z.Slice(q.Cursor()), mover.Slice(q.Cursor())
		for i := range zs {
			if movers[i].Domain == d {
				zs[i].Altitude = alt
			}
		}
	}
}

type slantRow struct {
	At  cell.ID
	Top float64
}

func TestSurface_ASlantwiseDeckCarriesOverTheCornerItCuts(t *testing.T) {
	const top, width, thick = 20.0, 8.0, 1.0
	define := func(qw *topotest.QuasiWorld, units *board.Units[topotest.Recruit]) []kind.Entry {
		nw, _ := grid.Direction(qw.Grid, -1, -1)
		se, _ := grid.Direction(qw.Grid, 1, 1)
		decks := board.NewEntities(qw.Board, board.Shape{Size: 20, Height: 1},
			func(r slantRow) cell.ID { return r.At })
		decks.Define("deck", comp.Load(func(r slantRow) cell.Floor {
			return cell.Floor{
				Allows:    cell.Land,
				Height:    r.Top - qw.Topo.Relief().Altitude(r.At),
				Thickness: thick,
				Links:     1<<nw | 1<<se,
				Width:     width,
			}
		}))
		var out []kind.Entry
		for _, c := range []cell.ID{qw.Grid.At(1, 2), qw.Grid.At(2, 3)} {
			out = append(out, decks.Named("deck").Entry(slantRow{At: c, Top: top}))
		}
		return out
	}
	qw := topotest.NewQuasiWorldWith(t, false, define)
	qw.ECS.Tick(time.Second / 60)
	surfaces := qw.Board.Surfaces()
	ground := qw.Topo.Relief()
	for _, c := range []struct {
		name string
		at   geom.Vec
		deck bool
	}{
		{"the corner it cuts", geom.NewVec(64, 96), true},
		{"by the corner, in the cell beside", geom.NewVec(64.5, 95), true},
		{"the middle of the cell beside", geom.NewVec(80, 80), false},
		{"beside it in its cell", geom.NewVec(56, 72), false},
	} {
		floor, _ := surfaces.Surface(c.at, cell.Land, top, 1)
		want := ground.At(c.at)
		if c.deck {
			want = top
		}
		if math.Abs(floor-want) > 1e-9 {
			t.Errorf("%s: a walker on the deck's level stands at %v, want %v", c.name, floor, want)
		}
	}
	_, ceiling := surfaces.Surface(geom.NewVec(64.5, 95), cell.Land, 0, 1)
	if ceiling != top-thick {
		t.Errorf("under it by the corner: a ceiling at %v, want the deck's underside %v",
			ceiling, top-thick)
	}
}
