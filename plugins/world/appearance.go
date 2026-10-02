package world

import "github.com/kjkrol/gram/render"

// Appearance is the sprite an entity is drawn from, and how much it bends in the wind: a
// render.Appearance, which the rules given to Plugin.Draw may layer over and an effect alter.
type Appearance = render.Appearance

// Facing picks every entity's sprite from the way it moves: a render.With over its Base, for
// Plugin.Draw.
func Facing(spriteFor func(Velocity) render.SpriteID) render.Rule {
	return render.With(func(a Appearance, b Base) Appearance { a.SpriteID = spriteFor(b.Vel); return a })
}
