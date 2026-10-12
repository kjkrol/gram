package game

import (
	"time"

	"github.com/kjkrol/goke/v3"
)

// Stage is one self-contained context a Game can be in, with its own plugins and ECS.
type Stage interface {
	// Name uniquely identifies this stage.
	Name() string

	// Init runs once, when this Stage becomes active.
	Init(ctx Initializer) error

	// Spawn runs once, when the Stage is entered afresh, seeding each plugin's initial state.
	Spawn() error

	// Loaded runs in place of Spawn when the Stage was entered from a save (Load), its world
	// loaded.
	Loaded() error

	// Update advances this stage's simulation by d, once per tick.
	Update(ctx goke.RunCtx, d time.Duration)

	// Stack lists every Scene this stage can show, and owns their live Composition.
	Stack() Scenes
}
