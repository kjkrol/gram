package main

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/buildings"
)

var (
	limewash  = color.RGBA{R: 238, G: 234, B: 224, A: 255}
	ochre     = color.RGBA{R: 232, G: 204, B: 148, A: 255}
	cream     = color.RGBA{R: 222, G: 214, B: 198, A: 255}
	concrete  = color.RGBA{R: 204, G: 206, B: 208, A: 255}
	sheeting  = color.RGBA{R: 188, G: 190, B: 188, A: 255}
	basalt    = color.RGBA{R: 132, G: 126, B: 120, A: 255}
	tiling    = color.RGBA{R: 186, G: 90, B: 58, A: 255}
	oldTiling = color.RGBA{R: 142, G: 72, B: 50, A: 255}
	terrace   = color.RGBA{R: 198, G: 194, B: 186, A: 255}
	metal     = color.RGBA{R: 150, G: 154, B: 158, A: 255}
)

// town is the kinds of buildings Funchal's file stands, every size in metres as in life: a
// building's length, width and storeys are its own, OpenStreetMap's, and a kind says how high a
// storey is, how many a building has where the map is silent, its roof and its colours.
type town struct{}

var _ buildings.Figure = town{}

func (town) Define(p *buildings.Plugin) {
	m := scale.Figure
	for name, spec := range map[string]buildings.Spec{
		// a whitewashed house under a hipped roof of tiles
		HouseKind: {
			Storey: m(3),
			Floors: 2,
			Walls:  limewash,
			Tiles:  tiling,
			Bay:    m(3),
			Parts:  []buildings.Part{{Roof: buildings.Hip, Pitch: 0.45}},
		},
		// a town house of the old streets, its gables to its neighbours
		TownhouseKind: {
			Storey: m(3.2),
			Floors: 3,
			Walls:  ochre,
			Tiles:  tiling,
			Bay:    m(2.6),
			Parts:  []buildings.Part{{Roof: buildings.Gable, Pitch: 0.4}},
		},
		VillaKind: {
			Storey: m(3),
			Floors: 2,
			Walls:  limewash,
			Tiles:  terrace,
			Bay:    m(3.5),
		},
		BlockKind: {
			Storey: m(3),
			Floors: 5,
			Walls:  cream,
			Tiles:  terrace,
			Bay:    m(3),
		},
		TowerKind: {
			Storey: m(3),
			Floors: 10,
			Walls:  concrete,
			Tiles:  terrace,
			Bay:    m(3),
		},
		// a hotel of the Lido, the lifts' room on its roof
		HotelKind: {
			Storey: m(3.2),
			Floors: 6,
			Walls:  limewash,
			Tiles:  terrace,
			Bay:    m(3.5),
			Parts: []buildings.Part{
				{},
				{Size: geom.NewVec(m(6), m(5)), Above: m(3), Walls: cream, Blind: true, Fine: true},
			},
		},
		// a parish church: a nave under a steep roof and a tower by its west front
		ChurchKind: {
			Storey: m(4),
			Floors: 3,
			Walls:  limewash,
			Tiles:  tiling,
			Bay:    m(5),
			Parts: []buildings.Part{
				{Roof: buildings.Gable, Pitch: 0.7},
				{
					Roof:  buildings.Pyramid,
					Pitch: 1.4,
					Size:  geom.NewVec(m(5), m(5)),
					Off:   geom.NewVec(-0.8, 0.5),
					Above: m(9),
					Tiles: basalt,
					Blind: true,
				},
			},
		},
		ChapelKind: {
			Storey: m(3.5),
			Floors: 2,
			Walls:  limewash,
			Tiles:  tiling,
			Bay:    m(4),
			Parts: []buildings.Part{
				{Roof: buildings.Gable, Pitch: 0.7},
				{
					Roof:  buildings.Pyramid,
					Pitch: 1.2,
					Size:  geom.NewVec(m(1.6), m(1.6)),
					Off:   geom.NewVec(-0.85, 0),
					Above: m(4),
					Tiles: basalt,
					Blind: true,
					Fine:  true,
				},
			},
		},
		// a warehouse, a works or a shop's hall under a low roof of sheets
		HallKind: {
			Storey: m(4),
			Floors: 2,
			Walls:  sheeting,
			Tiles:  metal,
			Parts:  []buildings.Part{{Roof: buildings.Gable, Pitch: 0.15}},
		},
		// a palheiro, a garage, a lean-to: stone under old tiles
		ShedKind: {
			Storey: m(2.5),
			Floors: 1,
			Walls:  basalt,
			Tiles:  oldTiling,
			Parts:  []buildings.Part{{Roof: buildings.Gable, Pitch: 0.35}},
		},
	} {
		p.Types().Define(name, spec)
	}
}
