package game

import "github.com/kjkrol/gram/camera"

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

	Persistence() Persistence
	TPS() *TPS

	// Camera returns the active Stage's world camera, or nil if the Stage has no world.
	Camera() camera.Camera

	// ToggleFullscreen switches the window to fullscreen and back; F11 does it in every game.
	ToggleFullscreen()
}
