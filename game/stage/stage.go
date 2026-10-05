package stage

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
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

// SpawnStep is what Layout and Units are given: a function seeding a fresh game.
type SpawnStep interface{ func() | func() error }

// def is a Stage as its sections define it.
type def struct {
	name    string
	steps   []step
	scenes  func(game.Initializer) ([]game.Scene, error)
	shows   []string
	restore func(game.Persistence) (bool, error)
	layout  func() error
	units   func() error
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

// Init runs the sections in their order, each told to the Initializer as it begins, then makes
// the stack of the scenes, shows the first — or those Shows named — and tracks its Composition.
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
	enter(ctx, section.Scenes)
	var scenes []game.Scene
	if d.scenes != nil {
		var err error
		if scenes, err = d.scenes(ctx); err != nil {
			return fmt.Errorf("stage %q, %v: %w", d.name, section.Scenes, err)
		}
	}
	first := len(scenes) > 0
	if p, ok := ctx.(interface{ PluginScenes() []game.Scene }); ok {
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
	return ctx.Track(composition)
}

func (b built) Restore(p game.Persistence) (bool, error) {
	if b.d.restore == nil {
		return false, nil
	}
	return b.d.restore(p)
}

// Spawn seeds a fresh game: the layout, then the units.
func (b built) Spawn() error {
	d := b.d
	defer enter(d.ctx, section.Done)
	for _, s := range []struct {
		part section.Part
		run  func() error
	}{{section.Layout, d.layout}, {section.Units, d.units}} {
		if s.run == nil {
			continue
		}
		enter(d.ctx, s.part)
		if err := s.run(); err != nil {
			return fmt.Errorf("stage %q, %v: %w", d.name, s.part, err)
		}
	}
	return nil
}

func (b built) Update(ctx goke.RunCtx, dt time.Duration) { b.d.update(ctx, dt) }

func (b built) Stack() game.Scenes { return b.d.stack }
