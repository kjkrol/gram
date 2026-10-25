package board_test

import (
	"math/bits"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
)

func deckWorld(t *testing.T, width float32) (collision.Field, grid.Grid) {
	t.Helper()
	g := grid.DefaultGrids{}.Square(5, 3, 10)
	west, _ := grid.Direction(g, -1, 0)
	east, _ := grid.Direction(g, 1, 0)
	floor := cell.Floor{Allows: cell.Land, Height: 10, Thickness: 1}
	if width > 0 {
		floor.Links, floor.Width = 1<<west|1<<east, width
	}
	grass := cell.Kind{Cost: 1, Allows: cell.Land}
	return floored(t, g, 50, 30, grass, floor, g.At(1, 1), g.At(2, 1), g.At(3, 1)), g
}

func floored(t *testing.T, g grid.Grid, width, height uint32, ground cell.Kind, floor cell.Floor,
	cells ...cell.ID) collision.Field {
	t.Helper()
	wd := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	p := board.NewPlugin(g, &cell.MultipleOccupancy{}, wd)
	p.Write().Board().SetAll(ground)
	bridges := board.NewEntities(p, board.Shape{Size: 10}, func(c cell.ID) cell.ID { return c })
	bridges.Define("bridge",
		comp.Const(floor),
		comp.Const(cell.Rail{Keeps: cell.Land, Height: 1}))
	for _, c := range cells {
		wd.Write().Seed(bridges.Named("bridge").Entry(c))
	}
	ctx := boardtest.NewInstallCtx()
	if err := wd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := wd.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx.ECS().Setup(ctx.Systems()...)
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		wd.RunPlan(rc, d)
		p.RunPlan(rc, d)
		rc.Sync()
		wd.Write().Clock().Replay(rc, d)
	})
	ctx.ECS().Tick(time.Second / 60)
	return p.Read(0).Cover().(collision.Field)
}

func solidAt(f collision.Field, band collision.Band) []collide.FieldBox {
	var out []collide.FieldBox
	f.Solid(world.Layers(cell.Land), band, geom.NewAABBAt(geom.NewVec(0, 0), 50, 30),
		func(fb collision.FieldBox) bool {
			out = append(out, fb)
			return true
		})
	return out
}

func wallsAt(f collision.Field, band collision.Band) []collide.FieldWall {
	var out []collide.FieldWall
	f.Walls(world.Layers(cell.Land), band, geom.NewAABBAt(geom.NewVec(0, 0), 50, 30),
		func(w collide.FieldWall) bool {
			out = append(out, w)
			return true
		})
	return out
}

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
		if w.A.Y != w.B.Y || f32.Abs(w.A.Y-15) != 3 {
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

func side(b geom.AABB) (float32, float32) {
	return b.BottomRight.X - b.TopLeft.X, b.BottomRight.Y - b.TopLeft.Y
}

func TestField_OverWaterOnlyTheBandTakesAWalker(t *testing.T) {
	water := cell.Kind{Cost: 1, Allows: cell.Water}
	land := world.Layers(cell.Land)
	floor := cell.Floor{Allows: cell.Land, Height: 10, Thickness: 1, Width: 6}
	g := grid.DefaultGrids{}.Square(5, 3, 10)
	west, _ := grid.Direction(g, -1, 0)
	east, _ := grid.Direction(g, 1, 0)
	floor.Links = 1<<west | 1<<east
	straight := floored(t, g, 50, 30, water, floor, g.At(1, 1), g.At(2, 1), g.At(3, 1))
	whole := floored(t, g, 50, 30, water, cell.Floor{Allows: cell.Land, Height: 10, Thickness: 1},
		g.At(1, 1), g.At(2, 1), g.At(3, 1))
	d := grid.DefaultGrids{}.Square(4, 4, 10)
	nw, _ := grid.Direction(d, -1, -1)
	se, _ := grid.Direction(d, 1, 1)
	floor.Links = 1<<nw | 1<<se
	slantwise := floored(t, d, 40, 40, water, floor, d.At(1, 1), d.At(2, 2))
	for _, c := range []struct {
		name  string
		field collision.Field
		box   geom.AABB
		want  float32
	}{
		{"on the band", straight, geom.NewAABBAt(geom.NewVec(23, 13), 4, 4), 0},
		{"pushed off its side", straight, geom.NewAABBAt(geom.NewVec(23, 17), 4, 4), 12},
		{"a deck over the cell, past it", whole, geom.NewAABBAt(geom.NewVec(23, 17), 4, 4), 4},
		{"on a slantwise band's corner", slantwise, geom.NewAABBAt(geom.NewVec(18, 18), 4, 4), 0},
		{"beside a slantwise band", slantwise, geom.NewAABBAt(geom.NewVec(10, 18), 2, 2), 4},
	} {
		if got := c.field.Overhang(land, c.box); f32.Abs(got-c.want) > f32.Eps(c.want) {
			t.Errorf("%s: overhang %v, want %v", c.name, got, c.want)
		}
	}
}
