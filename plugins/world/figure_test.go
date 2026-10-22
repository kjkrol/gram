package world_test

import (
	"image/color"
	"slices"
	"testing"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type box struct {
	ran   []string
	atlas *world.Atlas
}

func (b *box) Effects(w *world.Plugin) {
	b.ran = append(b.ran, "effects")
	w.Write().Effects().Define("boxed", effect.Spec{})
}

func (b *box) Rules(w *world.Plugin) {
	b.ran = append(b.ran, "rules")
	w.Roles().Define("boxy")
}

func (b *box) Define(w *world.Plugin) {
	b.ran = append(b.ran, "define")
	kind.Define[struct{}](w.Kinds(), "box", kind.Spec{
		comp.Const(world.Position{}), comp.Const(world.Velocity{}),
	})
}

func (b *box) Look(a *world.Atlas, w *world.Plugin) {
	b.ran = append(b.ran, "look")
	b.atlas = a
	a.Add(kind.Named[struct{}](w.Kinds(), "box"), 4, render.Solid(color.RGBA{}))
}

// a figure with no states or rules of its own
type plain struct{ ran []string }

func (p *plain) Define(*world.Plugin)             { p.ran = append(p.ran, "define") }
func (p *plain) Look(*world.Atlas, *world.Plugin) { p.ran = append(p.ran, "look") }

func TestFigures_RunEachPartInItsSectionAndLookOnTheAtlas(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	f, p := &box{}, &plain{}
	w.Figures(f, p)
	if len(f.ran) != 0 {
		t.Fatalf("noted, the figure ran %v; want nothing yet", f.ran)
	}
	for _, part := range []section.Part{section.Players, section.Effects, section.Rules,
		section.Commands, section.Kinds, section.Controls} {
		w.FiguresIn(part)
	}
	if want := []string{"effects", "rules", "define"}; !slices.Equal(f.ran, want) {
		t.Errorf("the figure ran %v, want %v, each in its section", f.ran, want)
	}
	if want := []string{"define"}; !slices.Equal(p.ran, want) {
		t.Errorf("the plain figure ran %v, want %v", p.ran, want)
	}
	if _, _, _, ok := w.Kinds().Lookup("box"); !ok {
		t.Error("the figure's kind is not defined")
	}
	a := w.NewAtlas()
	a.Looks()
	if f.ran[len(f.ran)-1] != "look" || f.atlas != a || p.ran[len(p.ran)-1] != "look" {
		t.Errorf("after Looks the figures ran %v and %v, want each looked on the atlas", f.ran, p.ran)
	}
}
