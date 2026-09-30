package island

import (
	"image/color"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/painter"
)

// Colors is how each kind of the island looks: the sea, the grounds, the running water, the roads
// and the forest.
var Colors = map[string]color.RGBA{
	"water":  {R: 40, G: 90, B: 170, A: 255},
	"earth":  {R: 110, G: 150, B: 75, A: 255},
	"sand":   {R: 215, G: 195, B: 140, A: 255},
	"rock":   {R: 130, G: 125, B: 120, A: 255},
	"brook":  {R: 90, G: 145, B: 205, A: 255},
	"stream": {R: 90, G: 145, B: 205, A: 255},
	"river":  {R: 90, G: 145, B: 205, A: 255},
	"ford":   {R: 90, G: 145, B: 205, A: 255},
	"road":   {R: 165, G: 135, B: 95, A: 255},
	"bridge": {R: 115, G: 85, B: 55, A: 255},
	"forest": {R: 30, G: 90, B: 45, A: 255},
}

// Kinds are the island's kinds, in their Colors, for a board's dictionary: the sea; the grounds
// — sand slow going, rock rough, a climb on top of either on a map in relief; running water laid
// across the ground — a brook stepped over, a stream waded through, a river crossed only at a ford;
// roads and the bridges carrying them over the water; and the forest, which nothing plants yet.
// Only a world with heights takes what stands on a cell: forest is how tall the forest stands, in
// world units, 0 on a flat map.
func Kinds(forest float64) []board.CellKind {
	trees := board.CellKind{Name: board.Named("forest"), Cost: 7.5, Allows: board.Land | board.Air, Veil: 0.6, Height: forest}.Costing(board.Air, 1)
	kinds := []board.CellKind{
		{Name: board.Named("water"), Cost: 1, Allows: board.Water | board.Air},
		board.CellKind{Name: board.Named("earth"), Cost: 2.5, Allows: board.Land | board.Air}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("sand"), Cost: 4, Allows: board.Land | board.Air}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("rock"), Cost: 3.25, Allows: board.Land | board.Air}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("brook"), Cost: 3.25, Allows: board.Land | board.Water | board.Air}.Costing(board.Water|board.Air, 1),
		board.CellKind{Name: board.Named("stream"), Cost: 5, Allows: board.Land | board.Water | board.Air}.Costing(board.Water|board.Air, 1),
		{Name: board.Named("river"), Cost: 1, Allows: board.Water | board.Air},
		board.CellKind{Name: board.Named("ford"), Cost: 6.25, Allows: board.Land | board.Water | board.Air}.Costing(board.Water|board.Air, 1),
		{Name: board.Named("road"), Cost: 1, Allows: board.Land | board.Air, Graded: true},
		{Name: board.Named("bridge"), Cost: 1, Allows: board.Land | board.Air, Graded: true},
		trees,
	}
	for i := range kinds {
		kinds[i].Color = Colors[kinds[i].Name.String()]
	}
	return kinds
}

// Style gives t the island's looks in relief: the sea glinting under the grounds, which blend into
// one another, and the running water running, taking on the sea's colour towards its mouth.
func Style(t *topography.Plugin) *topography.Plugin {
	return t.Style("water", painter.Style{Under: true, Shine: 0.9}).
		Style("earth", painter.Style{Spread: 0.3}).
		Style("sand", painter.Style{Spread: 0.35}).
		Style("rock", painter.Style{Spread: 0.25}).
		Style("brook", painter.Style{Shine: 0.9, Flow: 30, MixWith: "water"}).
		Style("stream", painter.Style{Shine: 0.9, Flow: 30, MixWith: "water"}).
		Style("river", painter.Style{Shine: 0.9, Flow: 22, MixWith: "water"}).
		Style("ford", painter.Style{Shine: 0.9, Flow: 22, MixWith: "water"})
}
