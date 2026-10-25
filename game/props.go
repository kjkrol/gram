package game

// Props configures the window and the tick rate, read once as the engine starts.
type Props struct {
	Title string // the window's: short
	// Description is what the game is and how it is played, shown over the list of shortcuts (K).
	Description               string
	TargetTPS                 int
	ScreenWidth, ScreenHeight int
	// Resizable lets the player resize the window, the screen then being the window. Off, the
	// screen stays ScreenWidth x ScreenHeight and a larger window scales it whole.
	Resizable bool
	// Saves is the directory the game keeps its saves in, a file a Stage and slot; empty, the one
	// the game runs in.
	Saves string
	// Workers is how many goroutines share the work a plugin splits, the game's own among them: 0
	// as many as there are CPUs, one left to the drawing where the ticks run while a frame is
	// drawn; 1 none.
	Workers int
	// Serial has every frame ticked before it is drawn, as GRAM_SERIAL does whatever it says. Off,
	// the next frame is ticked while a frame is drawn, the input shown a frame later.
	Serial bool
}
