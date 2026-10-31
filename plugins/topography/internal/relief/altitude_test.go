package relief_test

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
)

func TestAltitude_IsTheGroundUnderTheUnitPlusItsLift(t *testing.T) {
	var walker, hawk kind.Of[topotest.Recruit]
	qw := topotest.NewQuasiWorld(
		t,
		false,
		func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
			units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
			walker = units.Named("walker")
			units.Define(
				"hawk",
				unit.Mover{Domain: cell.Air, Lift: 40},
				steering.Steering{MaxSpeed: 10},
			)
			hawk = units.Named("hawk")
			onHill := grid.At(2, 1)
			onGrass := grid.At(0, 3)
			return []kind.Entry{
				walker.Entry(topotest.Recruit{Start: onHill}),
				hawk.Entry(topotest.Recruit{Start: onHill}),
				walker.Entry(topotest.Recruit{Start: onGrass}),
			}
		},
	)
	qw.ECS.Tick(time.Second / 60)

	zs := qw.Zs()
	if got := zs[walker.ID()]; len(got) != 2 || got[0].Height != 2 || got[1].Height != 2 {
		t.Fatalf("walkers' Z = %v, want two, each 2 tall", got)
	}
	alts := map[float32]bool{}
	for _, z := range zs[walker.ID()] {
		alts[z.Altitude] = true
	}
	onHill := qw.Grid.At(2, 1)
	hillGround := qw.Topo.Write().Relief().At(qw.Grid.CellCenter(onHill))
	if hillGround <= 0 || !alts[hillGround] || !alts[0] {
		t.Errorf(
			"walkers stand at %v, want one at the hill's ground %v and one at 0 on the grass",
			alts,
			hillGround,
		)
	}
	if got := zs[hawk.ID()]; len(got) != 1 || got[0].Altitude != hillGround+40 ||
		got[0].Height != 2 {
		t.Errorf(
			"hawk's Z = %v, want altitude %v (the hill plus its lift) and height 2",
			got,
			hillGround+40,
		)
	}
	if qw.Board.Read(0).Heights() != qw.Topo.Write().Relief() {
		t.Error("the board's heights are not the topography's relief")
	}
}

// a body bigger than a cell stands on the highest ground under its box, read at its middle and
// its corners, as its box is drawn flat over the slope; one within a cell, standing at the same
// middle, on the ground there
func TestAltitude_ABodyBiggerThanACellStandsOnTheHighestGroundUnderItsBox(t *testing.T) {
	var giant, walker kind.Of[topotest.Recruit]
	// the corner of (1,1), whose cells are flat, its box reaching up the slope to the hill at (2,1)
	middle := geom.NewVec(32, 32)
	define := func(qw *topotest.QuasiWorld, _ *board.Units[topotest.Recruit]) []kind.Entry {
		at := func(topotest.Recruit) geom.Vec { return middle }
		walks := steering.Steering{MaxSpeed: 10}
		big := board.NewUnits(qw.Board, board.Shape{Size: 60, Height: 4}, at)
		big.Define("giant", unit.Mover{Domain: cell.Land}, walks)
		giant = big.Named("giant")
		small := board.NewUnits(qw.Board, board.Shape{Size: 20, Height: 2}, at)
		small.Define("walker", unit.Mover{Domain: cell.Land}, walks)
		walker = small.Named("walker")
		row := topotest.Recruit{Start: qw.Grid.At(0, 0)}
		return []kind.Entry{giant.Entry(row), walker.Entry(row)}
	}
	qw := topotest.NewQuasiWorldUpTo(t, false, 60, define)
	qw.ECS.Tick(time.Second / 60)

	relief := qw.Topo.Write().Relief()
	under, highest := relief.At(middle), relief.At(middle)
	for _, corner := range []geom.Vec{{X: 2, Y: 2}, {X: 62, Y: 2}, {X: 2, Y: 62}, {X: 62, Y: 62}} {
		highest = max(highest, relief.At(corner))
	}
	if highest <= under {
		t.Fatalf("the ground under the giant's box rises to %v, under its middle to %v: no slope",
			highest, under)
	}
	zs := qw.Zs()
	if got := zs[giant.ID()]; len(got) != 1 || got[0].Altitude != highest {
		t.Errorf("the giant stands at %v, want on the highest ground under its box, %v", got,
			highest)
	}
	if got := zs[walker.ID()]; len(got) != 1 || got[0].Altitude != under {
		t.Errorf("the walker stands at %v, want on the ground under its middle, %v", got, under)
	}
}

// on a hex board the ground runs from a cell's middle to its corners: a unit up the slope to the
// hill stands on the ground under it, not on its cell's level
func TestAltitude_AUnitOnAHexSlopeStandsOnTheGroundUnderIt(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(4, 4, 32)
	hill, beside := g.CellCenter(g.At(2, 1)), g.CellCenter(g.At(1, 1))
	slope := geom.NewVec(beside.X+0.4*(hill.X-beside.X), beside.Y+0.4*(hill.Y-beside.Y))
	var walker kind.Of[topotest.Recruit]
	define := func(qw *topotest.QuasiWorld, _ *board.Units[topotest.Recruit]) []kind.Entry {
		units := board.NewUnits(qw.Board, board.Shape{Size: 20, Height: 2},
			func(topotest.Recruit) geom.Vec { return slope })
		units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
		walker = units.Named("walker")
		return []kind.Entry{walker.Entry(topotest.Recruit{Start: qw.Grid.At(1, 1)})}
	}
	qw := topotest.NewQuasiWorldOn(t, g, false, 20, define)
	qw.ECS.Tick(time.Second / 60)

	relief := qw.Topo.Write().Relief()
	ground := relief.At(slope)
	if level := relief.Altitude(g.At(1, 1)); ground < level+1 {
		t.Fatalf("the ground up the slope stands at %v, its cell's level at %v: no slope", ground,
			level)
	}
	if got := qw.Zs()[walker.ID()]; len(got) != 1 || f32.Abs(got[0].Altitude-ground) > 1e-4 {
		t.Errorf("the walker up the slope stands at %v, want on the ground under it, %v", got,
			ground)
	}
}

func TestAltitude_ABodyBiggerThanAHexStandsOnTheHighestGroundUnderItsBox(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(4, 4, 32)
	middle := g.CellCenter(g.At(1, 1)) // beside the hill, its box reaching up the slope
	var giant kind.Of[topotest.Recruit]
	define := func(qw *topotest.QuasiWorld, _ *board.Units[topotest.Recruit]) []kind.Entry {
		big := board.NewUnits(qw.Board, board.Shape{Size: 60, Height: 4},
			func(topotest.Recruit) geom.Vec { return middle })
		big.Define("giant", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
		giant = big.Named("giant")
		return []kind.Entry{giant.Entry(topotest.Recruit{Start: qw.Grid.At(1, 1)})}
	}
	qw := topotest.NewQuasiWorldOn(t, g, false, 60, define)
	qw.ECS.Tick(time.Second / 60)

	relief := qw.Topo.Write().Relief()
	under, highest := relief.At(middle), relief.At(middle)
	for _, d := range [4][2]float32{{-30, -30}, {30, -30}, {-30, 30}, {30, 30}} {
		if q := geom.NewVec(middle.X+d[0], middle.Y+d[1]); func() bool {
			_, ok := g.CellAt(q)
			return ok
		}() {
			highest = max(highest, relief.At(q))
		}
	}
	if highest <= under {
		t.Fatalf("the ground under the giant's box rises to %v, under its middle to %v: no slope",
			highest, under)
	}
	if got := qw.Zs()[giant.ID()]; len(got) != 1 || f32.Abs(got[0].Altitude-highest) > 1e-4 {
		t.Errorf("the giant stands at %v, want on the highest ground under its box, %v", got,
			highest)
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
	ground := qw.Topo.Write().Relief().At(qw.Grid.CellCenter(onHill))
	deck := qw.Topo.Write().Relief().Altitude(onHill) + 30
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
	deck := qw.Topo.Write().Relief().Altitude(qw.Grid.At(1, 2)) + 10
	step := func(alt float32) float32 {
		flown(qw, func(_ *world.Base, z *world.Z, _ *unit.Mover, _ *steering.Driven,
			_ *steering.Course) {
			z.Altitude = alt
		})
		qw.ECS.Tick(time.Second / 60)
		var got float32
		flown(qw, func(_ *world.Base, z *world.Z, _ *unit.Mover, _ *steering.Driven,
			_ *steering.Course) {
			got = z.Altitude
		})
		return got
	}
	reach := float32(2.0 / 4) // a quarter of the walker's Height
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
		qw.Board.Write().Board().Set(river, cell.Kind{Name: cell.Named("river"), Pace: 1,
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
	deck := qw.Topo.Write().Relief().Altitude(qw.Grid.At(1, 2)) + 10
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
	qw := topotest.NewQuasiWorld(
		t,
		false,
		func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
			units.Define(
				"hawk",
				unit.Mover{Domain: cell.Air, Lift: 40},
				steering.Steering{MaxSpeed: 10},
				comp.Const(steering.Driven{}),
			)
			hawk := units.Named("hawk")
			onGrass := grid.At(0, 3)
			return []kind.Entry{hawk.Entry(topotest.Recruit{Start: onGrass})}
		},
	)
	onHill := qw.Grid.At(2, 1)
	onGrass := qw.Grid.At(0, 3)
	put := func(c cell.ID) func(*world.Base) {
		return func(b *world.Base) {
			at := qw.Grid.CellCenter(c)
			b.Pos.AABB = plane.NewAABB(geom.NewVec(at.X-10, at.Y-10), 20, 20)
		}
	}
	step := func(fn func(b *world.Base, z *world.Z, m *unit.Mover, d *steering.Driven, st *steering.Course)) (alt, lift float32) {
		flown(qw, fn)
		qw.ECS.Tick(time.Second / 60)
		flown(
			qw,
			func(_ *world.Base, z *world.Z, m *unit.Mover, _ *steering.Driven, _ *steering.Course) {
				alt, lift = z.Altitude, m.Lift
			},
		)
		return alt, lift
	}
	hill := qw.Topo.Write().Relief().At(qw.Grid.CellCenter(onHill))
	if alt, _ := step(func(*world.Base, *world.Z, *unit.Mover, *steering.Driven, *steering.Course) {}); alt != 40 {
		t.Fatalf("steered from nowhere it flies at %v, want its lift 40 over the grass", alt)
	}
	if alt, lift := step(func(b *world.Base, _ *world.Z, _ *unit.Mover, d *steering.Driven, _ *steering.Course) {
		*d = steering.Driven{Flown: true}
		put(onHill)(b)
	}); alt != 40 ||
		lift != 40-hill {
		t.Errorf(
			"flown over the hill %v high: at %v, lift %v; want at 40, lift %v",
			hill,
			alt,
			lift,
			40-hill,
		)
	}
	if alt, _ := step(func(b *world.Base, _ *world.Z, _ *unit.Mover, _ *steering.Driven, _ *steering.Course) {
		put(onGrass)(b)
	}); alt != 40 {
		t.Errorf("flown back over the grass: at %v, want at 40, not falling with the ground", alt)
	}
	alt, _ := step(
		func(_ *world.Base, _ *world.Z, _ *unit.Mover, d *steering.Driven, st *steering.Course) {
			d.Climb, st.Speed, st.WantSpeed = 0.6, 10, 10
		},
	)
	if want := float32(40 + 10*0.75/60); f32.Abs(alt-want) > f32.Eps(want) {
		t.Errorf("flown at 10 a second at a rise of 0.6: at %v after a tick, want %v", alt, want)
	}
	if alt, _ := step(func(b *world.Base, _ *world.Z, m *unit.Mover, d *steering.Driven, st *steering.Course) {
		d.Climb, st.Speed, st.WantSpeed, m.Clearance = 0, 0, 0, 39
		put(onHill)(b)
	}); alt != hill+39 {
		t.Errorf(
			"flown over the hill with a clearance of 39: at %v, want %v, the clearance over the ground",
			alt,
			hill+39,
		)
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
	}); alt != hill+41 ||
		lift != 41 {
		t.Errorf(
			"let go over the hill: at %v, lift %v; want its lift 41 over the ground again",
			alt,
			lift,
		)
	}
}

func TestAltitude_AFlyerKeepsUnderItsCeiling(t *testing.T) {
	var hawk, low kind.Of[topotest.Recruit]
	qw := topotest.NewQuasiWorld(
		t,
		false,
		func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
			units.Define(
				"hawk",
				unit.Mover{Domain: cell.Air, Lift: 40, Ceiling: 41},
				steering.Steering{MaxSpeed: 10},
			)
			hawk = units.Named("hawk")
			units.Define(
				"low",
				unit.Mover{Domain: cell.Air, Lift: 40, Ceiling: 1},
				steering.Steering{MaxSpeed: 10},
			)
			low = units.Named("low")
			onHill := grid.At(2, 1)
			onGrass := grid.At(0, 3)
			return []kind.Entry{
				hawk.Entry(topotest.Recruit{Start: onHill}),
				hawk.Entry(topotest.Recruit{Start: onGrass}),
				low.Entry(topotest.Recruit{Start: onHill}),
			}
		},
	)
	qw.ECS.Tick(time.Second / 60)
	onHill := qw.Grid.At(2, 1)
	hill := qw.Topo.Write().Relief().At(qw.Grid.CellCenter(onHill))
	if hill <= 1 || hill+40 <= 41 {
		t.Fatalf("the hill stands %v high: too low to test the ceiling on", hill)
	}
	zs := qw.Zs()
	alts := map[float32]bool{}
	for _, z := range zs[hawk.ID()] {
		alts[z.Altitude] = true
	}
	if len(alts) != 2 || !alts[40] || !alts[41] {
		t.Errorf(
			"hawks fly at %v, want one at its lift 40 over the grass and one held to the ceiling 41 over the hill",
			alts,
		)
	}
	for _, z := range zs[low.ID()] {
		if z.Altitude != hill {
			t.Errorf(
				"a flyer whose ceiling lies under the hill flies at %v, want on the hill %v",
				z.Altitude,
				hill,
			)
		}
	}
}

func flown(
	qw *topotest.QuasiWorld,
	fn func(b *world.Base, z *world.Z, m *unit.Mover, d *steering.Driven, st *steering.Course),
) {
	var base goke.Comp[world.Base]
	var z goke.Comp[world.Z]
	var mover goke.Comp[unit.Mover]
	var driven goke.Comp[steering.Driven]
	var steer goke.Comp[steering.Course]
	var q *goke.Query
	qw.ECS.RegSys(
		goke.SystemFn{
			OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base, &z, &mover, &driven, &steer).Build() },
		},
	)
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i := range cur.IDs {
			fn(
				&base.Slice(cur)[i],
				&z.Slice(cur)[i],
				&mover.Slice(cur)[i],
				&driven.Slice(cur)[i],
				&steer.Slice(cur)[i],
			)
		}
	}
}

func standAt(qw *topotest.QuasiWorld, d cell.Domain, alt float32) {
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
	Top float32
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
				Height:    r.Top - qw.Topo.Write().Relief().Altitude(r.At),
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
	surfaces := qw.Board.Read(0).Footing()
	ground := qw.Topo.Write().Relief()
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
		if f32.Abs(floor-want) > f32.Eps(want) {
			t.Errorf("%s: a walker on the deck's level stands at %v, want %v", c.name, floor, want)
		}
	}
	_, ceiling := surfaces.Surface(geom.NewVec(64.5, 95), cell.Land, 0, 1)
	if ceiling != top-thick {
		t.Errorf("under it by the corner: a ceiling at %v, want the deck's underside %v",
			ceiling, top-thick)
	}
}

func TestAltitude_StandsUnitsOnAHexBoardWithoutAllocating(t *testing.T) {
	define := func(qw *topotest.QuasiWorld, units *board.Units[topotest.Recruit]) []kind.Entry {
		walks := steering.Steering{MaxSpeed: 10}
		units.Define("walker", unit.Mover{Domain: cell.Land}, walks)
		at := func(r topotest.Recruit) geom.Vec { return qw.Grid.CellCenter(r.Start) }
		big := board.NewUnits(qw.Board, board.Shape{Size: 60, Height: 4}, at)
		big.Define("giant", unit.Mover{Domain: cell.Land}, walks)
		return []kind.Entry{
			units.Named("walker").Entry(topotest.Recruit{Start: qw.Grid.At(1, 1)}),
			big.Named("giant").Entry(topotest.Recruit{Start: qw.Grid.At(2, 2)}),
		}
	}
	qw := topotest.NewQuasiWorldOn(t, grid.DefaultGrids{}.Hex(4, 4, 32), false, 60, define)
	for range 10 {
		qw.ECS.Tick(time.Second / 60)
	}
	if n := testing.AllocsPerRun(50, func() { qw.ECS.Tick(time.Second / 60) }); n != 0 {
		t.Errorf("a step of the units on a hex board allocates %v, want nothing", n)
	}
}
