package world

import "github.com/kjkrol/gram/render"

// Appearance is the sprite an entity is drawn from, and how much it bends in the wind — a tree, a
// reed, a flag: 0 not at all, 1 as far as the wind blows it; the Renderer's rules may layer
// over it, an effect alter it.
type Appearance struct {
	SpriteID render.SpriteID
	Sway     float32
}
