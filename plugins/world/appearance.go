package world

import (
	"math"

	"github.com/kjkrol/gram/render"
)

// Appearance is the sprite an entity is drawn from, and how much it bends in the wind: a
// render.Appearance, which the rules given to Plugin.Draw may layer over and an effect alter.
type Appearance = render.Appearance

// Facing picks every entity's sprite from the way it moves: a render.With over its Base, for
// Plugin.Draw.
func Facing(spriteFor func(Velocity) render.SpriteID) render.Rule {
	return render.With(func(a Appearance, b Base) Appearance { a.SpriteID = spriteFor(b.Vel); return a })
}

// Turning turns every entity's sprite the way it moves — Appearance.Angle from its Vel.Dir, the
// heading steering holds even standing — in the one convention of the engine (render.Arrow,
// bullet.Flight.Heading): degrees, 0 east, against the clock with the screen's y growing down.
// A render.With over its Base, for Plugin.Draw; a sprite is authored facing east, its content
// within the circle inscribed in its square box (see render.Appearance.Angle). One that never
// moved (a zero Dir) keeps the Angle its Appearance holds.
func Turning() render.Rule {
	return render.With(func(a Appearance, b Base) Appearance {
		if b.Vel.Dir.X != 0 || b.Vel.Dir.Y != 0 {
			a.Angle = float32(math.Atan2(-b.Vel.Dir.Y, b.Vel.Dir.X) * 180 / math.Pi)
		}
		return a
	})
}
