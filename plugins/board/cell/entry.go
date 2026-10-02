package cell

// Entry sets Cell to the kind named Kind — none, the board's Layout's Default kept — and gives it
// Tags, the game's tags of places it carries for good.
type Entry struct {
	Kind string
	Cell ID
	Tags Tags
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
