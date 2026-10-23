package bridges_test

import (
	"image/color"
	"slices"
	"testing"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type span struct{ ran []string }

var (
	_ bridges.Figure   = (*span)(nil)
	_ bridges.Stateful = (*span)(nil)
)

func (s *span) Effects(p *bridges.Plugin) {
	s.ran = append(s.ran, "effects")
	p.World().Write().Effects().Define("rotten", effect.Spec{})
}

func (s *span) Define(p *bridges.Plugin) {
	s.ran = append(s.ran, "define")
	p.Types().Define("span", bridges.Spec{Allows: cell.Land, Width: 6, Thickness: 1, Pier: 2})
}

func (s *span) Look(a *world.Atlas, p *bridges.Plugin) {
	s.ran = append(s.ran, "look")
	t := p.Types().Named("span")
	a.Add(t.Deck(), 8, render.Solid(color.RGBA{})).Lying()
	a.Add(t.Pier(), 8, render.Solid(color.RGBA{})).Upright()
}

func TestFigures_RunTheirPartsOnTheTypesAndLookOnTheWorldsAtlas(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 60},
		Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	col := collision.NewPlugin(w, m)
	brd := board.NewPlugin(grid.DefaultGrids{}.Square(10, 6, 10), &cell.MultipleOccupancy{}, w).
		WithMotion(m).
		WithCollision(col)
	p := bridges.NewPlugin(w, brd, bridges.Config{})
	f := &span{}
	p.Figures(f)
	for _, part := range []section.Part{section.Effects, section.Rules, section.Kinds} {
		w.FiguresIn(part)
	}
	if want := []string{"effects", "define"}; !slices.Equal(f.ran, want) {
		t.Errorf("the figure ran %v, want %v", f.ran, want)
	}
	if _, _, _, ok := w.Kinds().Lookup("span.deck"); !ok {
		t.Error("the bridge's deck is not the world's kind")
	}
	w.NewAtlas().Looks()
	if f.ran[len(f.ran)-1] != "look" {
		t.Errorf("the world's Looks: the figure ran %v, want it looked", f.ran)
	}
}
