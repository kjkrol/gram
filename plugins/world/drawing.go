package world

import (
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Appearance is the sprite an entity is drawn from, and how much it bends in the wind — a tree, a
// reed, a flag: 0 not at all, 1 as far as the wind blows it; the Renderer's rules may layer
// over it, an effect alter it.
type Appearance struct {
	SpriteID render.SpriteID
	Sway     float32
}

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
