package game

import "github.com/kjkrol/gram/control"

// TPS is the built-in measured-ticks-per-second counter.
type TPS struct{ Ticks int }

// Runtime gives a Stage or Scene engine-level control: pause, stage switching, save/load.
type Runtime interface {
	Paused() bool
	Pause()
	Resume()
	TogglePause()
	Quit()

	// SwitchStage transitions to the Stage with the given Name().
	SwitchStage(name string) error

	// Save saves the active Stage in the save slot, "" the quicksave, where Props.Saves says.
	Save(slot string) error
	// Load enters stage — the active one for "" — from its save slot at the start of the next
	// Update: the Stage built anew, its world loaded instead of spawned. A save not there is an
	// error, and nothing changes.
	Load(stage, slot string) error
	// Issue gives cmd as nobody to whoever carries it out in the active Stage — the engine its own
	// (Save, Load, Enter, Quit) — the way a key does.
	Issue(cmd control.Command) error

	Persistence() Persistence
	TPS() *TPS

	// ToggleFullscreen switches the window to fullscreen and back; F11 does it in every game.
	ToggleFullscreen()
	// Props are the game's, as it started: its title, its description.
	Props() Props
}
