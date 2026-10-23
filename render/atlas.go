package render

import "github.com/kjkrol/gram/render/internal/atlas"

// Atlas is an AtlasSource built by adding sprites, each at a size of its own, and then Close,
// which lays the sheet out and bakes it.
type Atlas = atlas.Atlas

// Canvas is a sprite's pixels as a SpriteDrawer paints them.
type Canvas = atlas.Canvas

// SpriteDrawer paints one sprite's size x size pixels into dst.
type SpriteDrawer = atlas.SpriteDrawer

// Sprited is whoever is drawn from one slot of the sheet: a kind's handle, or a bare SpriteID,
// which stands for itself.
type Sprited = atlas.Sprited

// NewAtlas starts an empty atlas.
func NewAtlas() *Atlas { return atlas.NewAtlas() }
