package board_test

import (
	"math"
	"math/bits"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
)

// deckWorld is a 5x3 board of grass in a world with heights, cells 10 wide, a bridge over its
// middle row's three middle cells: a deck 10 up and 1 thick, running east–west width wide (0: over
// the whole cell), railings 1 high along it.
func deckWorld(t *testing.T, width float64) (collision.Field, grid.Grid) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 30},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	g := grid.DefaultGrids{}.Square(5, 3, 10)
	p := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	p.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	west, _ := grid.Direction(g, -1, 0)
	east, _ := grid.Direction(g, 1, 0)
	floor := cell.Floor{Allows: cell.Land, Height: 10, Thickness: 1}
	if width > 0 {
		floor.Links, floor.Width = 1<<west|1<<east, width
	}
	bridges := board.NewEntities(p, board.Shape{Size: 10}, func(c cell.ID) cell.ID { return c })
	bridges.Define("bridge",
		comp.Const(floor),
		comp.Const(cell.Rail{Keeps: cell.Land, Height: 1}))
	for x := range uint32(3) {
		w.Seed(bridges.Named("bridge").Entry(g.At(x+1, 1)))
	}
	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx.ECS().Setup(ctx.Systems()...)
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		p.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	})
	ctx.ECS().Tick(time.Second / 60)
	return p.Cover().(collision.Field), g
}

// solidAt is what f holds solid over the whole board for a walker spanning band.
func solidAt(f collision.Field, band collision.Band) []collide.FieldBox {
	var out []collide.FieldBox
	f.Solid(world.Layers(cell.Land), band, geom.NewAABBAt(geom.NewVec(0, 0), 50, 30),
		func(fb collision.FieldBox) bool {
			out = append(out, fb)
			return true
		})
	return out
}

// wallsAt are the walls f holds over the whole board for a walker spanning band.
func wallsAt(f collision.Field, band collision.Band) []collide.FieldWall {
	var out []collide.FieldWall
	f.Walls(world.Layers(cell.Land), band, geom.NewAABBAt(geom.NewVec(0, 0), 50, 30),
		func(w collide.FieldWall) bool {
			out = append(out, w)
			return true
		})
	return out
}

// bands is what a deck's slab is hit by: a walker too tall to pass under it, not one who fits,
// nor one stepping up onto it from the deck's end, nor one standing on it.
var bands = []struct {
	name string
	band collision.Band
	want int
}{
	{"a walker 12 tall under it", collision.Band{Bottom: 0, Top: 12, Step: 0.5}, 3},
	{"a walker 2 tall under it", collision.Band{Bottom: 0, Top: 2, Step: 0.5}, 0},
	{"a step under the deck's top", collision.Band{Bottom: 9.7, Top: 11.7, Step: 0.5}, 0},
	{"a walker on it", collision.Band{Bottom: 10, Top: 12, Step: 0.5}, 0},
}

// A deck over the whole cell is a slab of the cell's box; no railing stands on it.
func TestField_ADeckStrikesWhoeverItDoesNotFitOver(t *testing.T) {
	f, _ := deckWorld(t, 0)
	for _, c := range bands {
		n := 0
		for _, fb := range solidAt(f, c.band) {
			if w, h := side(fb.Box); w == 10 && h == 10 {
				n++
			}
		}
		if n != c.want {
			t.Errorf("%s: %d slabs, want %d", c.name, n, c.want)
		}
	}
	if walls := wallsAt(f, collision.Band{Bottom: 10, Top: 12, Step: 0.5}); len(walls) != 0 {
		t.Errorf("a deck over the whole cell has %d walls, want none", len(walls))
	}
}

// A deck running as a band is a slab as wide along it, the ground beside it open.
func TestField_ABandStrikesWhoeverItDoesNotFitOver(t *testing.T) {
	f, _ := deckWorld(t, 6)
	for _, c := range bands {
		n := 0
		for _, w := range wallsAt(f, c.band) {
			if w.Half == 3 {
				n++
			}
		}
		if n != c.want {
			t.Errorf("%s: %d slabs, want %d", c.name, n, c.want)
		}
	}
	if boxes := solidAt(f, bands[0].band); len(boxes) != 0 {
		t.Errorf("a band holds %d cells solid, want none", len(boxes))
	}
}

// Railings stand along both edges of the band for a walker on it, their ends closed where the
// bridge runs on into the next cell and open at its ends; a walker under the deck or one leaping
// over their tops meets none.
func TestField_RailingsKeepAWalkerOnTheDeck(t *testing.T) {
	f, g := deckWorld(t, 6)
	rails := func(band collision.Band) []collide.FieldWall {
		var out []collide.FieldWall
		for _, w := range wallsAt(f, band) {
			if w.Half < 3 {
				out = append(out, w)
			}
		}
		return out
	}
	on := rails(collision.Band{Bottom: 10, Top: 12, Step: 0.5})
	if len(on) != 6 {
		t.Fatalf("a walker on the deck meets %d railings, want two edges of three cells", len(on))
	}
	for _, w := range on {
		if w.A.Y != w.B.Y || math.Abs(w.A.Y-15) != 3 {
			t.Errorf("a railing from %v to %v, want it along y 12 or 18", w.A, w.B)
		}
		want := collide.Ends(0)
		switch w.Cell {
		case uint64(g.At(1, 1)), uint64(g.At(3, 1)):
			want = 1 // one end free, the bridge's
		}
		if open := bitsOf(w.Open); open != int(want) {
			t.Errorf("the railing on cell %d has %d free ends, want %d", w.Cell, open, want)
		}
	}
	if under := rails(collision.Band{Bottom: 0, Top: 2, Step: 0.5}); len(under) != 0 {
		t.Errorf("a walker under the deck meets %d railings, want none", len(under))
	}
	if over := rails(collision.Band{Bottom: 11.5, Top: 13.5, Step: 0.5}); len(over) != 0 {
		t.Errorf("a walker leaping over the railings meets %d, want none", len(over))
	}
}

func bitsOf(e collide.Ends) int { return bits.OnesCount8(uint8(e)) }

func side(b geom.AABB) (float64, float64) {
	return b.BottomRight.X - b.TopLeft.X, b.BottomRight.Y - b.TopLeft.Y
}
