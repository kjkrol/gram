package board

import (
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// Atlas is the board's sprite sheet under construction: Add each kind's look by its name, chain
// Under for its covers, then Close it: the board takes it as its looks.
type Atlas struct {
	p      *Plugin
	atlas  *render.Atlas
	size   int
	shaded map[cell.Name]render.MaterialID
}

var _ render.AtlasSource = (*Atlas)(nil)

// NewAtlas starts the board's atlas for looks declared by hand, every sprite drawn size x size
// (the drawn size is the cell's box, this is resolution); Dress is the tiles' way.
func (p *Plugin) NewAtlas(size int) *Atlas {
	return &Atlas{p: p, atlas: render.NewAtlas(), size: size, shaded: map[cell.Name]render.MaterialID{}}
}

// Add takes look on as the look of the cell kind named kind: a SpriteDrawer drawn once into the
// sheet, or a MaterialID worked out per pixel every frame. An unknown kind panics by name.
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

// Under adds the cover laid over the cells under e, one drawer an effect.
func (s Slot) Under(e effect.Effect, draw render.SpriteDrawer) Slot {
	s.a.atlas.Add(s.a.p.Covering(e), s.a.size, draw)
	return s
}

// Close lays the sheet out, bakes it and hands it to the board (Plugin.SetAtlas); call once,
// after the last look.
func (a *Atlas) Close() {
	a.atlas.Close()
	a.p.SetAtlas(a)
}

// Atlas is the baked sheet.
func (a *Atlas) Atlas() *render.Image { return a.atlas.Atlas() }

// UV is a sprite's rectangle on the sheet.
func (a *Atlas) UV(id render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return a.atlas.UV(id) }

// White is the white texel plain colours sample.
func (a *Atlas) White() (u, v float32) { return a.atlas.White() }
