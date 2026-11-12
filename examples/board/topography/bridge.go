package main

import (
	"image/color"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var deckThickness = scale.Figure(0.2)

// bridge is a graded deck on piers, railed a metre high
type bridge struct{}

var _ bridges.Figure = bridge{}

func (bridge) Define(p *bridges.Plugin) {
	p.Types().Define(BridgeKind, bridges.Spec{
		Allows:    cell.Land,
		Width:     scale.Figure(2.5),
		Thickness: deckThickness,
		Pace:      1,
		Rail:      cell.Rail{Keeps: cell.Land, Height: scale.Figure(1)},
		Pier:      scale.Figure(0.625),
	})
}

func (bridge) Look(a *world.Atlas, p *bridges.Plugin) {
	t := p.Types().Named(BridgeKind)
	a.Add(t.Deck(), CellSize, deck).Lying().Edges(railing).
		Styled(minimap.Mark{Color: deckColor, Size: 4})
	a.Add(t.Pier(), CellSize, render.Solid(pierColor)).Upright()
}

// authored running east
func deck(dst *render.Canvas, size int) {
	w := float32(size)
	dst.Fill(deckColor)
	for i := range 8 {
		dst.FillRect(w*float32(i)/8, 0, w*0.02, w, railColor)
	}
}

func railing(dst *render.Canvas, size int) {
	w := float32(size)
	dst.FillRect(0, 0, w, w*0.2, railColor)
	for i := range 4 {
		dst.FillRect(w*(float32(i)+0.4)/4, 0, w*0.06, w, railColor)
	}
}

// span is a bridge of a town laid along its street's path, drawn as it is built, every size in
// metres as in life: a slab on wall piers, a stone arch, a girder on slender piers
type span struct {
	name string
	spec bridges.Spec
}

var _ bridges.Figure = span{}

var spans = []span{
	{name: BeamKind, spec: bridges.Spec{
		Model:   bridges.Beam,
		Girder:  scale.Units(1.2),
		Pier:    scale.Units(1),
		Span:    scale.Units(18),
		Body:    color.RGBA{R: 168, G: 166, B: 160, A: 255},
		Parapet: color.RGBA{R: 196, G: 194, B: 188, A: 255},
	}},
	{name: ArchKind, spec: bridges.Spec{
		Model:   bridges.Arch,
		Girder:  scale.Units(0.8),
		Pier:    scale.Units(1.6),
		Span:    scale.Units(12),
		Body:    color.RGBA{R: 122, G: 112, B: 104, A: 255},
		Parapet: color.RGBA{R: 140, G: 130, B: 120, A: 255},
	}},
	{name: ViaductKind, spec: bridges.Spec{
		Model:   bridges.Viaduct,
		Girder:  scale.Units(2.5),
		Pier:    scale.Units(2.5),
		Span:    scale.Units(40),
		Body:    color.RGBA{R: 178, G: 176, B: 170, A: 255},
		Parapet: color.RGBA{R: 200, G: 198, B: 192, A: 255},
	}},
	// a marina's pier: a walkway low over the water, no parapet
	{name: PontoonKind, spec: bridges.Spec{
		Model:   bridges.Beam,
		Girder:  scale.Units(0.4),
		Pier:    scale.Units(0.4),
		Span:    scale.Units(6),
		Body:    color.RGBA{R: 150, G: 132, B: 108, A: 255},
		Parapet: color.RGBA{R: 150, G: 132, B: 108, A: 255},
	}},
}

func (s span) Define(p *bridges.Plugin) {
	spec := s.spec
	spec.Allows = cell.Land
	spec.Width = scale.Units(7)
	spec.Thickness = scale.Units(1)
	spec.Pace = 1
	spec.Rail = cell.Rail{Keeps: cell.Land, Height: scale.Units(1.1)}
	if s.name == PontoonKind {
		spec.Width, spec.Thickness = scale.Units(3), scale.Units(0.4)
		spec.Rail = cell.Rail{}
	}
	p.Types().Define(s.name, spec)
}

func (span) Look(*world.Atlas, *bridges.Plugin) {}
