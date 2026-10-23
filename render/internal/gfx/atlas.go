package gfx

// SpriteID identifies a sprite in an Atlas.
type SpriteID uint16

// SpriteID is the slot itself.
func (id SpriteID) SpriteID() SpriteID { return id }

// AtlasSource supplies the sprite sheet a frame draws from: each sprite's rectangle on it, and a
// white texel plain colours sample.
type AtlasSource interface {
	Atlas() *Image
	UV(id SpriteID) (sx0, sy0, sx1, sy1 float32)
	White() (u, v float32)
}
