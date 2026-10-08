package island

import (
	"image/color"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/painter"
)

// Colors is how each kind of the island looks: the sea, the grounds, the running water, the roads
// and the forest.
var Colors = map[string]color.RGBA{
	WaterCell:  {R: 40, G: 90, B: 170, A: 255},
	EarthCell:  {R: 110, G: 150, B: 75, A: 255},
	SandCell:   {R: 215, G: 195, B: 140, A: 255},
	RockCell:   {R: 130, G: 125, B: 120, A: 255},
	BrookCell:  {R: 90, G: 145, B: 205, A: 255},
	StreamCell: {R: 90, G: 145, B: 205, A: 255},
	RiverCell:  {R: 90, G: 145, B: 205, A: 255},
	FordCell:   {R: 90, G: 145, B: 205, A: 255},
	RoadCell:   {R: 165, G: 135, B: 95, A: 255},
	BridgeCell: {R: 115, G: 85, B: 55, A: 255},
	ForestCell: {R: 30, G: 90, B: 45, A: 255},
}

// Kinds are the island's kinds, in their Colors, for a board's dictionary: the sea; the grounds
// — sand slow going, rock rough, a climb on top of either on a map in relief; running water laid
// across the ground — a brook stepped over, a stream waded through, a river crossed only at a ford;
// roads and the bridges carrying them over the water; and the forest, which nothing plants yet.
// Only a world with heights takes what stands on a cell: forest is how tall the forest stands, in
// world units, 0 on a flat map.
func Kinds(forest float64) []cell.Kind {
	trees := cell.Kind{Name: cell.Named(ForestCell), Cost: 7.5, Allows: cell.Land | cell.Air, Veil: 0.6, Height: forest}.Costing(cell.Air, 1)
	kinds := []cell.Kind{
		{Name: cell.Named(WaterCell), Cost: 1, Allows: cell.Water | cell.Air},
		cell.Kind{Name: cell.Named(EarthCell), Cost: 2.5, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1),
		cell.Kind{Name: cell.Named(SandCell), Cost: 4, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1),
		cell.Kind{Name: cell.Named(RockCell), Cost: 3.25, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1),
		cell.Kind{Name: cell.Named(BrookCell), Cost: 3.25, Allows: cell.Land | cell.Water | cell.Air}.Costing(cell.Water|cell.Air, 1),
		cell.Kind{Name: cell.Named(StreamCell), Cost: 5, Allows: cell.Land | cell.Water | cell.Air}.Costing(cell.Water|cell.Air, 1),
		{Name: cell.Named(RiverCell), Cost: 1, Allows: cell.Water | cell.Air},
		cell.Kind{Name: cell.Named(FordCell), Cost: 6.25, Allows: cell.Land | cell.Water | cell.Air}.Costing(cell.Water|cell.Air, 1),
		{Name: cell.Named(RoadCell), Cost: 1, Allows: cell.Land | cell.Air, Graded: true},
		{Name: cell.Named(BridgeCell), Cost: 1, Allows: cell.Land | cell.Air, Graded: true},
		trees,
	}
	for i := range kinds {
		kinds[i].Color = Colors[kinds[i].Name.String()]
	}
	return kinds
}

// Define registers the island's kinds with dict, in a Stage's Kinds section.
func Define(dict cell.Kinds, forest float64) {
	for _, k := range Kinds(forest) {
		dict.Define(k.Name.String(), k)
	}
}

// Style gives t the island's looks in relief: the sea glinting under the grounds, which blend into
// one another, and the running water running, taking on the sea's colour towards its mouth.
func Style(t *topography.Plugin) *topography.Plugin {
	return t.Style(WaterCell, painter.Style{Under: true, Shine: 0.9}).
		Style(EarthCell, painter.Style{Spread: 0.3}).
		Style(SandCell, painter.Style{Spread: 0.35}).
		Style(RockCell, painter.Style{Spread: 0.25}).
		Style(BrookCell, painter.Style{Shine: 0.9, Flow: 30, MixWith: WaterCell}).
		Style(StreamCell, painter.Style{Shine: 0.9, Flow: 30, MixWith: WaterCell}).
		Style(RiverCell, painter.Style{Shine: 0.9, Flow: 22, MixWith: WaterCell}).
		Style(FordCell, painter.Style{Shine: 0.9, Flow: 22, MixWith: WaterCell})
}
