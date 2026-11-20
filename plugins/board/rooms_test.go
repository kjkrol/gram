package board_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
)

// a house of two cells, (2, 2) and (3, 2), on ground falling 4 a cell southward, its floor as
// high as the ground in the middle of the door in the first one's southern edge: 12
type house struct {
	read  *board.Read
	g     grid.Grid
	a, b  cell.ID
	floor float32
}

const (
	houseClear float32 = 10
	houseFloor float32 = 12
)

type roomRow struct {
	At    cell.ID
	Links cell.Links
}

func housed(t *testing.T, door cell.Gap, ground cell.Kind) house {
	t.Helper()
	return housedWalled(t, door, ground, 0)
}

// housedWalled is housed, its walls walls thick
func housedWalled(t *testing.T, door cell.Gap, ground cell.Kind, walls float32) house {
	t.Helper()
	g := grid.DefaultGrids{}.Square(6, 6, 32)
	wd := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 192, Height: 192},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 32},
		Heights:  true,
	})
	p := board.NewPlugin(g, &cell.MultipleOccupancy{}, wd)
	topo := topography.NewPlugin(wd, p, nil, topography.Config{})
	p.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	topo.Write().Relief().SetHeights(func(q geom.Vec) float32 { return (192 - q.Y) / 8 })
	a, b := g.At(2, 2), g.At(3, 2)
	p.Write().Board().Set(a, ground)
	p.Write().Board().Set(b, ground)
	east, _ := grid.Direction(g, 1, 0)
	west, _ := grid.Direction(g, -1, 0)
	south, _ := grid.Direction(g, 0, 1)
	rooms := board.NewEntities(p, board.Shape{Size: 32}, func(r roomRow) cell.ID { return r.At })
	rooms.Define("room", comp.Load(func(r roomRow) cell.Floor {
		return cell.Floor{
			Allows:   cell.Land,
			Height:   houseFloor - topo.Write().Relief().Altitude(r.At),
			Pace:     1,
			Graded:   true,
			Links:    r.Links,
			Storey:   1,
			Clear:    houseClear,
			Walls:    walls,
			Standing: true,
			Door:     door,
		}
	}))
	wd.Write().Seed(rooms.Named("room").Entry(roomRow{At: a, Links: 1<<east | 1<<south}))
	wd.Write().Seed(rooms.Named("room").Entry(roomRow{At: b, Links: 1 << west}))
	ctx := boardtest.NewInstallCtx()
	for _, install := range []func() error{
		func() error { return wd.Install(ctx) },
		func() error { return p.Install(ctx) },
		func() error { return topo.Install(ctx) },
	} {
		if err := install(); err != nil {
			t.Fatal(err)
		}
	}
	if err := wd.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx.ECS().Setup(ctx.Systems()...)
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		wd.RunPlan(rc, d)
		p.RunPlan(rc, d)
		topo.RunPlan(rc, d)
		rc.Sync()
		wd.Write().Clock().Replay(rc, d)
	})
	ctx.ECS().Tick(time.Second / 60)
	ctx.ECS().Tick(time.Second / 60)
	return house{read: p.Read(0), g: g, a: a, b: b, floor: houseFloor}
}

var (
	grassKind = cell.Kind{Pace: 1, Allows: cell.Land}
	frontDoor = cell.Gap{Width: 4, Height: 8}
	inRoom    = cell.OnDeck + 1
)

func TestRoom_StandingOnASlopeItsFloorIsLevelAndItsGroundShut(t *testing.T) {
	h := housed(t, frontDoor, grassKind)
	brd := h.read.Board()
	// by the northern wall the ground lies 3 over the floor, by the southern one 1 under it
	for _, p := range []geom.Vec{{X: 66, Y: 66}, {X: 80, Y: 94}, {X: 126, Y: 66}, {X: 110, Y: 94}} {
		floor, ceiling := h.read.Footing().Surface(p, cell.Land, h.floor, 2)
		if floor != h.floor || ceiling != h.floor+houseClear {
			t.Errorf("at %v the floor %v and the roof %v, want %v and %v", p, floor, ceiling,
				h.floor, h.floor+houseClear)
		}
		if _, level, _ := brd.LevelFor(p, h.floor, 2); level != inRoom {
			t.Errorf("at %v whoever stands on the floor is on %v, want the room's %v", p, level,
				inRoom)
		}
		if _, level, _ := brd.LevelFor(p, f32.Inf(1), 2); level != inRoom {
			t.Errorf("a pick from above at %v lands on %v, want the room's %v", p, level, inRoom)
		}
		c, _ := h.g.CellAt(p)
		if brd.KindFor(c, p, h.floor-6, 2).Admits(cell.Land) {
			t.Errorf("at %v the ground under the floor takes a walker", p)
		}
		if !brd.KindFor(c, p, h.floor, 2).Admits(cell.Land) {
			t.Errorf("at %v the room's floor takes no walker", p)
		}
	}
	street := geom.NewVec(80, 110)
	if c, level, _ := brd.LevelFor(street, 11, 2); c != h.g.At(2, 3) || level != cell.OnGround {
		t.Errorf("before the door the walker is on %v of %v, want the ground of the next cell",
			level, c)
	}
	if floor, _ := h.read.Footing().Surface(street, cell.Land, 10, 2); floor != 10.25 {
		t.Errorf("the ground before the door lies at %v, want the slope's 10.25", floor)
	}
	b := brd.Built(h.a)
	if f := b.Storey(1); f.Height != -2 || !f.Room() || f.Dug() {
		t.Errorf("the house's floor lies %v off its cell's level, a room %v, dug %v; want 2 "+
			"under it, a room standing", f.Height, f.Room(), f.Dug())
	}
}

func TestRoom_ReachedAlongItsLinksAndOutByItsDoorAlone(t *testing.T) {
	h := housed(t, frontDoor, grassKind)
	brd := h.read.Board()
	for _, c := range []struct {
		name     string
		from, to cell.ID
		want     bool
	}{
		{"its other cell", h.a, h.b, true},
		{"out by the door", h.a, h.g.At(2, 3), true},
		{"out by the northern wall", h.a, h.g.At(2, 1), false},
		{"out by the other cell's southern wall", h.b, h.g.At(3, 3), false},
	} {
		if got := brd.ReachesOn(c.from, c.to, inRoom); got != c.want {
			t.Errorf("%s: the room reaches %v, want %v", c.name, got, c.want)
		}
	}
	if _, gap := brd.Limits(h.a, cell.Land, inRoom); gap != frontDoor {
		t.Errorf("the room limits whoever walks it by %v, want its door %v", gap, frontDoor)
	}
	shut := housed(t, cell.Gap{}, grassKind)
	if shut.read.Board().ReachesOn(shut.a, shut.g.At(2, 3), inRoom) {
		t.Error("a room whose door is shut reaches out by it")
	}
}

func roomWalls(h house, band collision.Band) []collide.FieldWall {
	var out []collide.FieldWall
	h.read.Cover().(collision.Field).Walls(world.Layers(cell.Land), band,
		geom.NewAABBAt(geom.NewVec(48, 48), 96, 64), func(w collide.FieldWall) bool {
			out = append(out, w)
			return true
		})
	return out
}

// wallOver says whether a wall of walls runs over the point p of an edge
func wallOver(walls []collide.FieldWall, p geom.Vec) bool {
	for _, w := range walls {
		lo := geom.NewVec(min(w.A.X, w.B.X)-w.Half, min(w.A.Y, w.B.Y)-w.Half)
		hi := geom.NewVec(max(w.A.X, w.B.X)+w.Half, max(w.A.Y, w.B.Y)+w.Half)
		if p.X >= lo.X && p.X <= hi.X && p.Y >= lo.Y && p.Y <= hi.Y {
			return true
		}
	}
	return false
}

func TestRoom_WalledRoundItsCellsButForItsDoor(t *testing.T) {
	h := housed(t, frontDoor, grassKind)
	walls := roomWalls(h, collision.Band{Bottom: h.floor, Top: h.floor + 6, Step: 2})
	for _, c := range []struct {
		name string
		at   geom.Vec
		want bool
	}{
		{"the first cell's northern edge", geom.NewVec(72, 65), true},
		{"its western edge", geom.NewVec(65, 80), true},
		{"its southern edge beside the door", geom.NewVec(70, 95), true},
		{"outside its southern edge", geom.NewVec(70, 97), false},
		{"the door", geom.NewVec(80, 95), false},
		{"the edge between its two cells", geom.NewVec(96, 80), false},
		{"the other cell's eastern edge", geom.NewVec(127, 80), true},
		{"the other cell's southern edge", geom.NewVec(112, 95), true},
	} {
		if got := wallOver(walls, c.at); got != c.want {
			t.Errorf("%s at %v walled %v, want %v", c.name, c.at, got, c.want)
		}
	}
	if over := roomWalls(h, collision.Band{Bottom: 30, Top: 36, Step: 2}); len(over) > 0 {
		t.Errorf("a body over the roof meets %d of the room's walls, want none", len(over))
	}
}

func TestRoom_StandingItsWallsStandInsideItsCellsAsThickAsItsFloorSaysItsDoorAsWide(t *testing.T) {
	h := housedWalled(t, frontDoor, grassKind, 3)
	walls := roomWalls(h, collision.Band{Bottom: h.floor, Top: h.floor + 6, Step: 2})
	for _, c := range []struct {
		name string
		at   geom.Vec
		want bool
	}{
		{"its western wall by its inner face", geom.NewVec(66.9, 80), true},
		{"the room past that face", geom.NewVec(67.1, 80), false},
		{"outside its western edge", geom.NewVec(63.9, 80), false},
	} {
		if got := wallOver(walls, c.at); got != c.want {
			t.Errorf("%s at %v walled %v, want %v", c.name, c.at, got, c.want)
		}
	}
	// the door's jambs end its width apart round its middle, 80, free for a body clipping one
	var jambs []float32
	for _, w := range walls {
		if w.Half != 1.5 {
			t.Errorf("a wall %v thick, want the floor's 3", 2*w.Half)
		}
		if w.Open&collide.EndA != 0 {
			jambs = append(jambs, w.A.X)
		}
		if w.Open&collide.EndB != 0 {
			jambs = append(jambs, w.B.X)
		}
	}
	if slices.Sort(jambs); !slices.Equal(jambs, []float32{78, 82}) {
		t.Errorf("the walls' free ends at %v, want the door's jambs at 78 and 82", jambs)
	}
}

// walkedInto is a flat board of 6 x 6 cells of 32 under a house of one cell, (2, 2), its walls
// walls thick, its door as door says in the middle of its southern edge, and a unit of mover, 20
// tall, keeping on its way
func walkedInto(t *testing.T, g grid.Grid, walls float32, door cell.Gap,
	mover boardtest.Mover) *boardtest.World {
	t.Helper()
	var links cell.Links
	if south, _ := grid.Direction(g, 0, 1); door.Width > 0 {
		links = 1 << south
	}
	mover.Z = world.Z{Height: 20}
	return boardtest.NewWorldIn(t, g, 192, 192,
		func(cfg *world.Config) { cfg.Entities.MaxSize = 32 },
		func(w *world.Plugin, p *board.Plugin) []act.Rule {
			p.Write().Board().SetAll(grassKind)
			rooms := board.NewEntities(p, board.Shape{Size: 32},
				func(r roomRow) cell.ID { return r.At })
			rooms.Define("room", comp.Const(cell.Floor{
				Allows:   cell.Land,
				Pace:     1,
				Graded:   true,
				Links:    links,
				Storey:   1,
				Clear:    houseClear,
				Walls:    walls,
				Standing: true,
				Door:     door,
			}))
			w.Write().Seed(rooms.Named("room").Entry(roomRow{At: g.At(2, 2)}))
			return nil
		}, []boardtest.Mover{mover})
}

// walker is the box of walkedInto's unit, its kind defined after the room's
func walker(bw *boardtest.World) geom.AABB {
	boxes := bw.Snapshot()
	return boxes[len(boxes)-1]
}

func TestRoom_AUnitStopsAtAStandingRoomsWallsFacesFromOutsideAndInsideWithoutBouncing(
	t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, boardtest.CellSize)
	for _, c := range []struct {
		name  string
		from  cell.ID
		stops float32 // where the unit's eastern side comes to rest, the wall's face
	}{
		{"from outside", g.At(0, 2), 64},
		{"from inside", g.At(2, 2), 96 - 3},
	} {
		t.Run(c.name, func(t *testing.T) {
			bw := walkedInto(t, g, 3, cell.Gap{},
				boardtest.Mover{Here: c.from, Heading: geom.NewVec(1, 0)})
			most := f32.Inf(-1)
			for tick := range 90 {
				bw.Tick()
				side := walker(bw).BottomRight.X
				if side < most-0.01 {
					t.Fatalf("tick %d: the unit thrown back to %v from %v", tick, side, most)
				}
				most = max(most, side)
			}
			if f32.Abs(most-c.stops) > 0.01 {
				t.Errorf("the unit's side came to %v, want it at the wall's face, %v", most,
					c.stops)
			}
		})
	}
}

func TestRoom_AUnitWalksInByItsDoorToTheFarWall(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, boardtest.CellSize)
	// the door 26 wide round 80, the unit 22 wide a step off its middle
	bw := walkedInto(t, g, 3, cell.Gap{Width: 26, Height: 18},
		boardtest.Mover{Here: g.At(2, 3), Heading: geom.NewVec(0, -1), Offset: 1.5})
	for range 120 {
		bw.Tick()
	}
	if top := walker(bw).TopLeft.Y; f32.Abs(top-(64+3)) > 0.01 {
		t.Errorf("the unit's front at %v, want it in at the northern wall's face, 67", top)
	}
}

func roomBoxes(h house, band collision.Band) map[cell.ID]bool {
	out := map[cell.ID]bool{}
	h.read.Cover().(collision.Field).Solid(world.Layers(cell.Land), band,
		geom.NewAABBAt(geom.NewVec(48, 48), 96, 64), func(fb collision.FieldBox) bool {
			out[cell.ID(fb.Cell)] = true
			return true
		})
	return out
}

func TestRoom_ItsFoundationIsSolidUnderItsFloorWhateverTheGround(t *testing.T) {
	for name, ground := range map[string]cell.Kind{
		"grass":          grassKind,
		"ground none on": {Solid: true},
	} {
		h := housed(t, frontDoor, ground)
		in := roomBoxes(h, collision.Band{Bottom: h.floor, Top: h.floor + 6, Step: 2})
		if in[h.a] || in[h.b] {
			t.Errorf("%s: a body on the floor strikes the house's cells", name)
		}
		under := roomBoxes(h, collision.Band{Bottom: h.floor - 8, Top: h.floor - 2, Step: 2})
		if !under[h.a] || !under[h.b] {
			t.Errorf("%s: a body under the floor strikes the cells %v, want both the house's",
				name, under)
		}
	}
}
