// Package engine is the driver behind gram.Run: the window's loop (gogpu), one active Stage and
// its ECS at a time, and the unexported implementations of the contracts package game declares.
// Nothing here is a user's business to read; the public surface is package game.
//
// # Engine
//
// [Engine] runs a game.Game in a window: every frame a tick of the active Stage and a picture of
// its visible scenes. It is also the game.Runtime every Stage and Scene sees. GRAM_FPS_LOG has it
// log the frame rate every second, GRAM_FULLSCREEN starts it fullscreen (F11 switches),
// GRAM_VSYNC=off unpaces it.
//
// # Initializer, runtime and persistence
//
// The unexported initializer is the game.Initializer a Stage's Init receives, the unexported
// persistence the game.Persistence over the active Stage: a save is the ECS snapshot plus every
// tracked value's Persisted state, matched by name on load, so a save survives plugins being
// added, removed or reordered.
//
// # Input
//
// An [InputAdapter] captures one frame's raw input into control.InputEvents; [DesktopAdapter] is
// the window's. [DefaultController] runs the capture each frame and hands the events to its
// handler every tick; [HandlerFn] adapts a plain function.
package engine
