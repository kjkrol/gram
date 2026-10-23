package board_test

import (
	"image/color"
	"slices"
	"testing"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type post struct{ ran []string }

func (p *post) Effects(b *board.Plugin) {
	p.ran = append(p.ran, "effects")
	b.Read(0).World().Write().Effects().Define("posted", effect.Spec{})
}

func (p *post) Define(b *board.Plugin) {
	p.ran = append(p.ran, "define")
	board.NewEntities(b, board.Shape{Size: 4}, itself).Define("post", comp.Const(cell.Wall{}))
}

func (p *post) Look(a *world.Atlas, w *world.Plugin) {
	p.ran = append(p.ran, "look")
	a.Add(kind.Named[cell.ID](w.Kinds(), "post"), 4, render.Solid(color.RGBA{}))
}

type mud struct{ ran []string }

func (m *mud) Rules(b *board.Plugin) {
	m.ran = append(m.ran, "rules")
	b.Read(0).World().Roles().Define("muddy")
}

func (m *mud) Define(b *board.Plugin) {
	m.ran = append(m.ran, "define")
	b.Read(0).CellKinds().Define("mud", cell.Kind{Cost: 3, Allows: cell.Land}, b.Read(0).World().Roles().Named("muddy"))
}

func (m *mud) Look(a *board.Atlas, _ *board.Plugin) {
	m.ran = append(m.ran, "look")
	a.Add("mud", render.Solid(color.RGBA{}))
}

func TestFiguresAndTiles_RunTheirPartsOverTheBoardAndLookOnTheirAtlases(t *testing.T) {
	g := grid.DefaultGrids{}.Square(3, 3, boardtest.CellSize)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 3 * boardtest.CellSize, Height: 3 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 16},
		Heights:  true,
	})
	b := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	tile, f := &mud{}, &post{}
	b.Tiles(tile)
	b.Figures(f)
	for _, part := range []section.Part{section.Effects, section.Rules, section.Kinds} {
		w.FiguresIn(part)
	}
	if want := []string{"effects", "define"}; !slices.Equal(f.ran, want) {
		t.Errorf("the figure ran %v, want %v", f.ran, want)
	}
	if want := []string{"rules", "define"}; !slices.Equal(tile.ran, want) {
		t.Errorf("the tile ran %v, want %v", tile.ran, want)
	}
	if _, _, _, ok := w.Kinds().Lookup("post"); !ok {
		t.Error("the figure's kind is not the world's")
	}
	b.Read(0).CellKinds().Named("mud") // the tile's kind is the board's: unknown, it panics by name
	w.NewAtlas().Looks()
	if f.ran[len(f.ran)-1] != "look" || tile.ran[len(tile.ran)-1] == "look" {
		t.Errorf("the world's Looks: figure %v, tile %v; want the figure looked, not the tile", f.ran, tile.ran)
	}
	b.Surfaces(surfaced(func(*board.Atlas, *board.Plugin) {
		if tile.ran[len(tile.ran)-1] != "look" {
			t.Errorf("a surface looked before the tile, which ran %v", tile.ran)
		}
		tile.ran = append(tile.ran, "surface")
	}))
	b.NewAtlas(8).Looks()
	if tile.ran[len(tile.ran)-1] != "surface" {
		t.Errorf("the board's Looks: tile %v, want it looked and the surface after it", tile.ran)
	}
}
