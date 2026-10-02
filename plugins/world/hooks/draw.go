package hooks

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Overlay draws with on top of every entity carrying T.
func Overlay[T any](with world.Appearance) plugin.Rule {
	return host.Each(func(_ plugin.Tick, _ *T, d world.Drawing) { d.Overlay(with) })
}

// As draws every entity carrying T as with, in place of its own sprite.
func As[T any](with world.Appearance) plugin.Rule {
	return host.Each(func(_ plugin.Tick, _ *T, d world.Drawing) { d.As(with) })
}

// With reworks the sprite of every entity carrying T through fn, which sees the T it carries.
func With[T any](fn func(world.Appearance, T) world.Appearance) plugin.Rule {
	return host.Each(func(_ plugin.Tick, t *T, d world.Drawing) {
		d.With(func(a world.Appearance) world.Appearance { return fn(a, *t) })
	})
}

// Facing picks every entity's sprite from the way it moves.
func Facing(spriteFor func(world.Velocity) render.SpriteID) plugin.Rule {
	return host.Every(func(_ plugin.Tick, d world.Drawing) {
		d.With(func(a world.Appearance) world.Appearance { a.SpriteID = spriteFor(d.Base.Vel); return a })
	})
}
