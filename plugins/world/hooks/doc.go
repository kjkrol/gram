// Package hooks holds ready-made rules of drawing, for the world plugin's Hook: they change how an
// entity is drawn this frame — its sprite swapped, reworked or laid over — and leave its
// Appearance as it is. A state that changes the Appearance itself, for as long as it lasts, is an
// effect (effect.Alter); a rule here is what is drawn, whatever the state.
//
// The rules of a world.Drawing run in the order hooked, each on the layers the one before left:
// hook Facing before As, so an entity drawn as something else is not turned back.
//
// # Overlay, As and With
//
// [Overlay] draws a sprite on top of every entity carrying a component, [As] draws such an entity
// as another sprite in place of its own, and [With] reworks its sprite through a function that
// sees the component — a value read every frame, so the sprite follows it.
//
// # Facing
//
// [Facing] picks every entity's sprite from the way it moves.
package hooks
