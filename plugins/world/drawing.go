package world

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Drawing is what a rule hosted by the Renderer gets for every entity about to be
// drawn: Layers start as its Appearance and each rule may change them, in order.
type Drawing struct {
	ID     uid.UID64
	Base   *Base
	Layers *[]Appearance
}

// Who is the entity drawn: whose moment it is, for a rule.
func (d Drawing) Who() uid.UID64 { return d.ID }

// Overlay draws a on top of what is there.
func (d Drawing) Overlay(a Appearance) { *d.Layers = append(*d.Layers, a) }

// As draws a in place of the entity's own sprite.
func (d Drawing) As(a Appearance) { (*d.Layers)[0] = a }

// With reworks the entity's own sprite through fn.
func (d Drawing) With(fn func(Appearance) Appearance) { (*d.Layers)[0] = fn((*d.Layers)[0]) }

// Draw is the namespace of the ready-made drawing hooks: hang them on the world plugin.
var Draw draw

type draw struct{}

// Overlay draws with on top of every entity carrying T.
func (draw) Overlay[T any](with Appearance) plugin.Rule {
	return host.Each(func(_ plugin.Tick, _ *T, d Drawing) { d.Overlay(with) })
}

// As draws every entity carrying T as with, in place of its own sprite.
func (draw) As[T any](with Appearance) plugin.Rule {
	return host.Each(func(_ plugin.Tick, _ *T, d Drawing) { d.As(with) })
}

// With reworks the sprite of every entity carrying T through fn, which sees the T it carries.
func (draw) With[T any](fn func(Appearance, T) Appearance) plugin.Rule {
	return host.Each(func(_ plugin.Tick, t *T, d Drawing) {
		d.With(func(a Appearance) Appearance { return fn(a, *t) })
	})
}

// Facing picks every entity's sprite from the way it moves.
func (draw) Facing(spriteFor func(Velocity) render.SpriteID) plugin.Rule {
	return host.Every(func(_ plugin.Tick, d Drawing) {
		d.With(func(a Appearance) Appearance { a.SpriteID = spriteFor(d.Base.Vel); return a })
	})
}
