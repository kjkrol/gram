package draw

import (
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/render"
)

// Cover is what lies on the cells under an effect: the sprite laid over each while the effect's
// marker is on it.
type Cover struct {
	Mark   tag.Tag[effect.States]
	Sprite render.SpriteID
}

// Cover has sprite laid over the cells under the effect marked mark.
func (d *Bands) Cover(mark tag.Tag[effect.States], sprite render.SpriteID) {
	d.covers = append(d.covers, Cover{Mark: mark, Sprite: sprite})
}
