package board

import (
	"testing"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

// a knob another plugin keeps on the cells
type lantern struct{ Lit float32 }

func (lantern) Knob() {}

var _ effect.Knob = lantern{}

func TestMattering_IsTheEffectsTheTerrainChangesUnder(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 8}, Heights: true})
	p := NewPlugin(grid.DefaultGrids{}.Square(4, 4, 32), &cell.MultipleOccupancy{}, w)
	fx := w.Write().Effects()
	fx.Define("snowed", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind.Cost = 9 })})
	fx.Define("lit", effect.Spec{effect.Alter(func(l *lantern) { l.Lit = 1 })})
	fx.Define("marked", effect.Spec{})
	fx.Define("frosted", effect.Spec{effect.Alter(func(l *lantern) { l.Lit = 0 })})
	p.Covering(fx.Named("frosted"))
	matters := p.mattering()
	for name, want := range map[string]bool{"snowed": true, "lit": false, "marked": true,
		"frosted": true} {
		if got := matters.Has(fx.Named(name).Mark()); got != want {
			t.Errorf("the terrain changes under %q %v, want %v", name, got, want)
		}
	}
}
