package stage

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
)

// Step is what a section is given: a function defining that part of the Stage, with the
// Initializer and an error where it needs them.
type Step interface {
	func() | func() error | func(game.Initializer) | func(game.Initializer) error
}

// ScenesStep is what Scenes is given: a function making the Stage's scenes.
type ScenesStep interface {
	func() []game.Scene | func() ([]game.Scene, error) | func(game.Initializer) []game.Scene | func(game.Initializer) ([]game.Scene, error)
}

// SpawnStep is what Spawn is given: a function seeding a fresh game.
type SpawnStep interface{ func() | func() error }

// def is a Stage as its sections define it.
type def struct {
	name    string
	steps   []step
	scenes  func(game.Initializer) ([]game.Scene, error)
	shows   []string
	restore func(game.Persistence) (bool, error)
	spawn   []func() error
	update  func(goke.RunCtx, time.Duration)

	ctx   game.Initializer
	stack game.Scenes
}

type step struct {
	part section.Part
	run  func(game.Initializer) error
}

func (d *def) add(part section.Part, run func(game.Initializer) error) {
	d.steps = append(d.steps, step{part: part, run: run})
}

// stepOf is f as a section runs it.
func stepOf[F Step](f F) func(game.Initializer) error {
	switch g := any(f).(type) {
	case func():
		return func(game.Initializer) error { g(); return nil }
	case func() error:
		return func(game.Initializer) error { return g() }
	case func(game.Initializer):
		return func(ctx game.Initializer) error { g(ctx); return nil }
	case func(game.Initializer) error:
		return g
	}
	panic("stage: unreachable")
}

func spawnOf[F SpawnStep](f F) func() error {
	switch g := any(f).(type) {
	case func():
		return func() error { g(); return nil }
	case func() error:
		return g
	}
	panic("stage: unreachable")
}

// enter tells ctx which part begins, where it listens.
func enter(ctx game.Initializer, p section.Part) {
	if w, ok := ctx.(section.Writer); ok {
		w.Enter(p)
	}
}

// built is the game.Stage a chain of sections ends in.
type built struct{ d *def }

var _ game.Stage = built{}

func (b built) Name() string { return b.d.name }

// Init runs the sections before the world — Plugins to Controls — in their order, each told to
// the Initializer as it begins.
func (b built) Init(ctx game.Initializer) error {
	d := b.d
	d.ctx = ctx
	defer enter(ctx, section.Done)
	for _, s := range d.steps {
		enter(ctx, s.part)
		if err := s.run(ctx); err != nil {
			return fmt.Errorf("stage %q, %v: %w", d.name, s.part, err)
		}
	}
	return nil
}

// Restore resumes the Stage from a save, as its Restore says, and makes the scenes of the game
// loaded; false, and the engine spawns a fresh one.
func (b built) Restore(p game.Persistence) (bool, error) {
	d := b.d
	if d.restore == nil {
		return false, nil
	}
	loaded, err := d.restore(p)
	if err != nil || !loaded {
		return loaded, err
	}
	return true, d.makeScenes()
}

// Spawn seeds a fresh game — its steps in their order — and makes its scenes.
func (b built) Spawn() error {
	d := b.d
	defer enter(d.ctx, section.Done)
	enter(d.ctx, section.Spawn)
	for _, run := range d.spawn {
		if err := run(); err != nil {
			return fmt.Errorf("stage %q, %v: %w", d.name, section.Spawn, err)
		}
	}
	return d.makeScenes()
}

// makeScenes makes the scenes once the world is there, fresh or loaded: the stack of the game's
// and the plugins' own, the first shown — or those Shows named — and the Composition and every
// scene keeping state of its own tracked for the saves, a loaded game's laid on them as tracked.
func (d *def) makeScenes() error {
	defer enter(d.ctx, section.Done)
	enter(d.ctx, section.Scenes)
	var scenes []game.Scene
	if d.scenes != nil {
		var err error
		if scenes, err = d.scenes(d.ctx); err != nil {
			return fmt.Errorf("stage %q, %v: %w", d.name, section.Scenes, err)
		}
	}
	first := len(scenes) > 0
	if p, ok := d.ctx.(interface{ PluginScenes() []game.Scene }); ok {
		scenes = append(scenes, p.PluginScenes()...) // the plugins' own, hidden until shown
	}
	stack, err := game.NewStack(scenes...)
	if err != nil {
		return fmt.Errorf("stage %q: %w", d.name, err)
	}
	d.stack = stack
	composition := stack.Composition()
	shows := d.shows
	if shows == nil && first {
		shows = []string{scenes[0].Name()}
	}
	for _, name := range shows {
		composition.Show(name)
	}
	for _, sc := range scenes {
		if s, ok := sc.(plugin.Serializable); ok { // a scene keeping state of its own: a ui scene's elements shown
			if err := d.ctx.Track(s); err != nil {
				return err
			}
		}
	}
	return d.ctx.Track(composition)
}

func (b built) Update(ctx goke.RunCtx, dt time.Duration) { b.d.update(ctx, dt) }

func (b built) Stack() game.Scenes { return b.d.stack }
