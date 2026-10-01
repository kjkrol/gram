package board

import "github.com/kjkrol/gram/plugins/world/entity/tag"

// Layout is a board's initial terrain for Plugin.Seed: Default fills every cell, then each
// CellEntry overrides one; each of Ways lays a Way across a cell, each of Crossings a Crossing over
// a cell's way. The ground's heights are a topography's to seed (plugins/topography).
type Layout struct {
	Default   string
	Cells     []CellEntry
	Ways      []WayEntry
	Crossings []WayEntry
}

// WayEntry lays a Way of the CellKind named Kind across Cell, Width wide, running on as Links says,
// faded out as far as Fade, its look turned as far as Mix.
type WayEntry struct {
	Kind  string
	Cell  CellID
	Width float32
	Links Links
	Fade  float32
	Mix   float32
}

// CellEntry sets Cell to the CellKind named Kind — none, the Default kept — and gives it Tags, the
// game's tags of places it carries for good (Places).
type CellEntry struct {
	Kind string
	Cell CellID
	Tags tag.Tags[Places]
}
