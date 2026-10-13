package game

import (
	"github.com/kjkrol/ceremony/command"
)

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
	// Load enters stage ("" the active one) from its save slot, its world loaded instead of
	// spawned. A save not there is an error, and nothing changes.
	Load(stage, slot string) error
	// Issue gives cmd as nobody to whoever carries it out in the active Stage, the way a key does.
	Issue(cmd command.Command) error

	Persistence() Persistence
	TPS() *TPS

	// ToggleFullscreen switches the window to fullscreen and back.
	ToggleFullscreen()
	// Props are the game's, as it started.
	Props() Props
}
