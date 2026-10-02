package game

// Props configures Engine's window and target tick rate — returned by
// Game.Props and read once, when the engine starts.
type Props struct {
	Title                     string
	TargetTPS                 int
	ScreenWidth, ScreenHeight int
	// Resizable lets the player resize and maximize the window: the screen then is the window,
	// every world camera draws to all of it — more of a large world, a small one scaled up to cover
	// it. Off, the screen stays ScreenWidth x ScreenHeight and a larger window scales it whole.
	Resizable bool
}
