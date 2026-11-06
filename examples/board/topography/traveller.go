package main

import (
	"image/color"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// how long a traveller stays at either end of its way
const travellerRest = 5 * time.Second

type travellerRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

// traveller is a walker going to and fro between two places, resting at either: seen from afar
// a figure on a disc.
type traveller struct{ color color.RGBA }

var _ board.Figure = traveller{}

func (traveller) Define(b *board.Plugin) {
	w := b.Read(0).World()
	units := board.NewUnits[travellerRow](b,
		board.Shape{Size: scale.Figure(0.6), Height: figure},
		func(r travellerRow) geom.Vec { return b.Read(0).Board().CellCenter(r.Start) })
	extra := append(looking(),
		ceremony.Plays(w.Roles().Named(MortalRole)),
		leaping(),
		comp.Load(func(r travellerRow) navigation.MoveOrder {
			return navigation.MoveOrder{Target: r.Target}
		}),
		comp.Load(func(r travellerRow) navigation.Round {
			return navigation.NewRound(r.Target, r.Start)
		}),
		shuttle(TravellerPlan, travellerRest))
	units.Define(TravellerKind, unit.Mover{Domain: cell.Land}, walking, extra...)
}

func (t traveller) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[travellerRow](w.Kinds(), TravellerKind), iconPx, render.Solid(t.color)).
		Upright().
		Styled(selection.Highlight{Line: t.color, Width: 2}).
		Styled(minimap.Mark{Color: t.color, Size: 5}).
		At(lod.L4, travellerIcon(t.color))
}
