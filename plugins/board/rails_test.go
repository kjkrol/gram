package board_test

import (
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
// middle row's three middle cells: a deck 10 up and 1 thick, railings 1 high along its north and
// south sides.
func deckWorld(t *testing.T) (collision.Field, grid.Grid) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 30},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	g := grid.DefaultGrids{}.Square(5, 3, 10)
	p := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	p.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	bridges := board.NewEntities(p, board.Shape{Size: 10}, func(c cell.ID) cell.ID { return c })
	bridges.Define("bridge",
		comp.Const(cell.Floor{Allows: cell.Land, Height: 10, Thickness: 1}),
		comp.Const(cell.Rail{Keeps: cell.Land, Sides: 1 | 2, Height: 1}))
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

// A deck's slab strikes a walker too tall to pass under it, not one who fits, nor one stepping up
// onto it from the deck's end, nor one standing on it.
func TestField_ADeckStrikesWhoeverItDoesNotFitOver(t *testing.T) {
	f, _ := deckWorld(t)
	slabs := func(band collision.Band) int {
		n := 0
		for _, fb := range solidAt(f, band) {
			if w, h := side(fb.Box); w == 10 && h == 10 {
				n++
			}
		}
		return n
	}
	for _, c := range []struct {
		name string
		band collision.Band
		want int
	}{
		{"a walker 12 tall under it", collision.Band{Bottom: 0, Top: 12, Step: 0.5}, 3},
		{"a walker 2 tall under it", collision.Band{Bottom: 0, Top: 2, Step: 0.5}, 0},
		{"a step under the deck's top", collision.Band{Bottom: 9.7, Top: 11.7, Step: 0.5}, 0},
		{"a walker on it", collision.Band{Bottom: 10, Top: 12, Step: 0.5}, 0},
	} {
		if got := slabs(c.band); got != c.want {
			t.Errorf("%s: %d slabs, want %d", c.name, got, c.want)
		}
	}
}

// Railings stand along the deck's sides for a walker on it, their ends closed where they run on
// into the next cell; a walker under the deck or one leaping over their tops meets none.
func TestField_RailingsKeepAWalkerOnTheDeck(t *testing.T) {
	f, g := deckWorld(t)
	rails := func(band collision.Band) []collide.FieldBox {
		var out []collide.FieldBox
		for _, fb := range solidAt(f, band) {
			if w, h := side(fb.Box); w < 10 || h < 10 {
				out = append(out, fb)
			}
		}
		return out
	}
	on := rails(collision.Band{Bottom: 10, Top: 12, Step: 0.5})
	if len(on) != 6 {
		t.Fatalf("a walker on the deck meets %d railings, want two sides of three cells", len(on))
	}
	for _, fb := range on {
		middle := fb.Cell == uint64(g.At(2, 1))
		if ends := fb.Open & (collide.Left | collide.Right); middle && ends != 0 {
			t.Errorf("the middle cell's railing opens its ends %04b, want them closed", ends)
		}
	}
	if under := rails(collision.Band{Bottom: 0, Top: 2, Step: 0.5}); len(under) != 0 {
		t.Errorf("a walker under the deck meets %d railings, want none", len(under))
	}
	if over := rails(collision.Band{Bottom: 11.5, Top: 13.5, Step: 0.5}); len(over) != 0 {
		t.Errorf("a walker leaping over the railings meets %d, want none", len(over))
	}
}

func side(b geom.AABB) (float64, float64) {
	return b.BottomRight.X - b.TopLeft.X, b.BottomRight.Y - b.TopLeft.Y
}
