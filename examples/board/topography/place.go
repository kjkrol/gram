package main

import (
	"os"
	"strings"

	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world"
)

// place is a map the demo plays and the sizes it is played at.
type place struct {
	file       string
	cols, rows uint32
	// a cell's side, metres, and how many times bigger than life the figures are drawn
	metres  float32
	figures float32
	// the narrowest and the widest a thing stands, world units, and the space's buckets a
	// sector's side, which the widest sizes
	least   uint32
	widest  uint32
	buckets uint32
	// how far a unit sees and the minimap reaches, metres
	sees  float32
	reach float32
	zoom  float32
	// the trees' crowns as wide as in life, where the island's world holds them narrower
	crowns bool
	// its settlements stand the buildings its file lists, each its own
	town bool
}

var places = map[string]place{
	// the island whole: a cell 100 m, the figures 15 times bigger than life
	MadeiraMap: {
		file:    "map.yaml",
		cols:    640,
		rows:    320,
		metres:  100,
		figures: 15,
		least:   2,
		widest:  CellSize,
		buckets: sectorCells * CellSize / 64,
		sees:    10_000,
		reach:   12_000,
		zoom:    0.15,
	},
	// the town from the harbour up to Monte: a cell 10 m, everything as in life
	FunchalMap: {
		file:    "funchal.yaml",
		cols:    360,
		rows:    460,
		metres:  10,
		figures: 1,
		least:   1,
		widest:  6 * CellSize,
		buckets: sectorCells * CellSize / 512,
		sees:    1_000,
		reach:   1_500,
		zoom:    0.3,
		crowns:  true,
		town:    true,
	},
}

// here is the place of -map, read off the command line before the flags are parsed: the sizes
// the package's variables are worked out of as the program loads hang on it.
var here = func() place {
	name := MadeiraMap
	for i, arg := range os.Args[1:] {
		arg = strings.TrimLeft(arg, "-")
		if v, ok := strings.CutPrefix(arg, "map="); ok {
			name = v
		} else if arg == "map" && i+2 < len(os.Args) {
			name = os.Args[i+2]
		}
	}
	s, ok := places[name]
	if !ok {
		println("no map is named " + name + ": madeira or funchal")
		os.Exit(2)
	}
	return s
}()

var (
	GridWidth   = here.cols
	GridHeight  = here.rows
	WorldWidth  = GridWidth * CellSize
	WorldHeight = GridHeight * CellSize
)

var scale = world.Scale{Metres: here.metres / CellSize, Figures: here.figures}

// the ladder's scales: L2 from a tree of 12 pixels, L3 down to 8, L4, the sheet, under that
var ladder = [lod.Levels - 1]float32{0, 0, 0.6, 0.4, 0.12, 0}
