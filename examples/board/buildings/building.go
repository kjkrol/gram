package main

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/buildings"
)

// homes is the kinds of buildings of the village, their sizes in metres as in life: a cottage
// under a hipped roof, a house of a street under a gabled one, a hall under a flat one with a
// room on it, each with a door where its line gives it one, and a chapel with its tower under
// a pyramid.
type homes struct{}

var _ buildings.Figure = homes{}

// a door of the village, 1.1 m wide and 2.2 m tall
var doorway = cell.Gap{Width: scale.Units(1.1), Height: scale.Units(2.2)}

func (homes) Define(p *buildings.Plugin) {
	m := scale.Units
	p.Types().Define(CottageKind, buildings.Spec{
		Storey: m(3),
		Floors: 1,
		Size:   geom.NewVec(m(7), m(5.5)),
		Walls:  limeColor,
		Washes: []buildings.Wash{
			{Colour: limeColor, Share: 55},
			{Colour: creamColor, Share: 25},
			{Colour: ochreColor, Share: 12},
			{Colour: pinkColor, Share: 8},
		},
		Stone: stoneColor,
		Tiles: tileColor,
		Bay:   m(2),
		Parts: []buildings.Part{{Roof: buildings.Hip, Pitch: 0.5}},
		Door:  doorway,
	})
	p.Types().Define(TownhouseKind, buildings.Spec{
		Storey: m(3),
		Floors: 3,
		Size:   geom.NewVec(m(7.5), m(6)),
		Walls:  ochreColor,
		Stone:  stoneColor,
		Tiles:  tileColor,
		Bay:    m(1.5),
		Parts:  []buildings.Part{{Roof: buildings.Gable, Pitch: 0.45}},
		Door:   doorway,
	})
	p.Types().Define(HallKind, buildings.Spec{
		Storey: m(4),
		Floors: 1,
		Size:   geom.NewVec(m(14), m(7)),
		Walls:  brickColor,
		Tiles:  gravelColor,
		Bay:    m(2.5),
		Parts: []buildings.Part{
			{Roof: buildings.Flat},
			{
				Roof:  buildings.Flat,
				Size:  geom.NewVec(m(4), m(3)),
				Off:   geom.NewVec(0.5, 0),
				Above: m(2.5),
				Blind: true,
			},
		},
		Door: doorway,
	})
	p.Types().Define(ChapelKind, buildings.Spec{
		Storey: m(4.5),
		Floors: 1,
		Size:   geom.NewVec(m(12), m(6)),
		Walls:  limeColor,
		Stone:  stoneColor,
		Tiles:  tileColor,
		Bay:    m(3),
		Parts: []buildings.Part{
			{Roof: buildings.Gable, Pitch: 0.8},
			{
				Roof:  buildings.Pyramid,
				Pitch: 1.4,
				Size:  geom.NewVec(m(2.6), m(2.6)),
				Off:   geom.NewVec(-0.75, 0),
				Above: m(4),
				Tiles: slateColor,
				Blind: true,
			},
		},
	})
}
