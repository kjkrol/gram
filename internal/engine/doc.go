// Package engine is the driver behind gram.Run: the window's loop (gogpu), one active Stage and its
// ECS at a time, and the unexported implementations of the contracts package game declares.
// Nothing here is a user's business to read; the public surface is package game.
//
// # Engine
//
// [Engine] runs a game.Game in a window: Run opens it through gogpu and, every frame, calls Update
// and Draw and presents the picture (Layout says the screen's size). Init enters the initial
// Stage; each Update handles the active Scene's input, then ticks the Stage's ECS at the game's
// TargetTPS — at most five steps when a frame falls behind — and tells the world's clock the time
// held toward the next step, for what is drawn to move every frame; Draw runs the visible Scenes'
// layers in composition order, each viewport into an image of its own. GRAM_FPS_LOG has it log the
// frame rate and where a frame's time goes every second; GRAM_FULLSCREEN starts it fullscreen (F11
// switches), GRAM_VSYNC=off unpaces it; on Wayland it asks gogpu not to wait for the compositor
// after every frame. SwitchStage is made at the start of the next Update: the
// new Stage's ECS and world are built then, its Init, Restore and Spawn run, and its plugins'
// wiring is flushed in one ecs.Setup. Engine is also the game.Runtime every Stage and Scene sees.
//
// # Initializer, runtime and persistence
//
// The unexported initializer is the game.Initializer a Stage's Init receives: it collects what
// Use, Track, UseWorld and the plugin.Installer methods ask for and hands it to the ECS once. The
// unexported persistence implements game.Persistence over the active Stage: a save is the ECS
// snapshot plus every tracked value's Persisted pointers, gob-encoded under a base path and label,
// written through a temp file; resources are matched by name on load, so a save survives plugins
// being added, removed or reordered. The unexported storage is the name-keyed registry behind it.
//
// # Input
//
// An [InputAdapter] captures one frame's raw input into control.InputEvents; [DesktopAdapter] is
// gogpu's window's. [DefaultController] runs the capture each frame and, once a handler is set,
// hands the events to it every tick; [HandlerFn] adapts a plain function.
package engine
