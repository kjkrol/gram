package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule/effect"
)

// Atlas is the board's sprite sheet under construction, written as the world's atlas is: Add each
// kind's look by its name, chain Under for what covers its cells under an effect, then Close it
// and hand it to WithRenderer — it is the render.AtlasSource the renderer draws from.
type Atlas struct {
	p      *Plugin
	atlas  *render.Atlas
	size   int
	shaded map[cell.Name]render.MaterialID // the kinds worked out per pixel instead of drawn
}

var _ render.AtlasSource = (*Atlas)(nil)

// NewAtlas starts the board's atlas, every sprite drawn size x size — the drawn size is the
// cell's box; this is resolution.
func (p *Plugin) NewAtlas(size int) *Atlas {
	return &Atlas{p: p, atlas: render.NewAtlas(), size: size, shaded: map[cell.Name]render.MaterialID{}}
}

// Add takes look on as the look of the cell kind named kind; chain Under for the covers. look
// paints one of the two ways (render.Look): a SpriteDrawer drawn once into the sheet, or a
// MaterialID worked out per pixel in the cell's box every frame — never baked into the still,
// and taking no part in blending (give such a kind no Spread). An unknown kind panics by name.
func (a *Atlas) Add[L render.Look](kind string, look L) Slot {
	of := a.p.kinds.Named(kind)
	switch l := any(look).(type) {
	case render.MaterialID:
		a.shaded[cell.Named(kind)] = l
	case render.SpriteDrawer:
		a.atlas.Add(of, a.size, l)
	case func(dst *render.Canvas, size int):
		a.atlas.Add(of, a.size, l)
	}
	return Slot{a: a}
}

// Slot is one kind's look added to the board's Atlas.
type Slot struct{ a *Atlas }

// Under adds the cover laid over the cells under e — snow on the ground, ice on the water: one
// drawer an effect, laid along the line those cells draw (Covering) whatever their kind; the
// chain says where the game expects it.
func (s Slot) Under(e effect.Effect, draw render.SpriteDrawer) Slot {
	s.a.atlas.Add(s.a.p.Covering(e), s.a.size, draw)
	return s
}

// Close lays the sheet out and bakes it; call once, after the last Add.
func (a *Atlas) Close() { a.atlas.Close() }

// Atlas is the baked sheet — see render.AtlasSource.
func (a *Atlas) Atlas() *render.Image { return a.atlas.Atlas() }

// UV is a sprite's rectangle on the sheet — see render.AtlasSource.
func (a *Atlas) UV(id render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return a.atlas.UV(id) }

// White is the white texel plain colours sample — see render.AtlasSource.
func (a *Atlas) White() (u, v float32) { return a.atlas.White() }
