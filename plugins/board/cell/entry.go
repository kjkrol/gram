package cell

import "github.com/kjkrol/gram/rule"

// Entry sets Cell to the kind named Kind — none, the board's Layout's Default kept — and gives it,
// for good, the Roles it plays, the Name it bears, its alone, and the Group it is in with others:
// what commands find it by (entity.Named, entity.Group).
type Entry struct {
	Kind  string
	Cell  ID
	Roles []*rule.Part
	Name  string
	Group string
}

// WayEntry lays a Way of the kind named Kind across Cell, Width wide, running on as Links says,
// faded out as far as Fade, its look turned as far as Mix.
type WayEntry struct {
	Kind  string
	Cell  ID
	Width float32
	Links Links
	Fade  float32
	Mix   float32
}
