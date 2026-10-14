package bullet_test

import (
	"image/color"
	"slices"
	"testing"

	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type pellet struct{ ran []string }

var (
	_ bullet.Figure = (*pellet)(nil)
	_ bullet.Ruled  = (*pellet)(nil)
)

func (p *pellet) Rules(b *bullet.Plugin) {
	p.ran = append(p.ran, "rules")
	b.World().Roles().Define("stinging")
}

func (p *pellet) Define(b *bullet.Plugin) {
	p.ran = append(p.ran, "define")
	b.Shots().Define("pellet", bullet.Body{Size: 2, Speed: 10, Range: 20})
}

func (p *pellet) Look(a *world.Atlas, b *bullet.Plugin) {
	p.ran = append(p.ran, "look")
	a.Add(b.Shots().Named("pellet"), 4, render.Solid(color.RGBA{})).Turning()
}

func TestFigures_RunTheirPartsOnTheShotsAndLookOnTheWorldsAtlas(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 10},
	})
	b := bullet.NewPlugin(w, selection.NewPlugin(w))
	f := &pellet{}
	b.Figures(f)
	for _, part := range []section.Part{section.Effects, section.Rules, section.Kinds} {
		w.FiguresIn(part)
	}
	if want := []string{"rules", "define"}; !slices.Equal(f.ran, want) {
		t.Errorf("the figure ran %v, want %v", f.ran, want)
	}
	if _, _, _, ok := w.Kinds().Lookup("pellet"); !ok {
		t.Error("the ammo is not the world's kind")
	}
	w.NewAtlas().Looks()
	if f.ran[len(f.ran)-1] != "look" {
		t.Errorf("the world's Looks: the figure ran %v, want it looked", f.ran)
	}
}
