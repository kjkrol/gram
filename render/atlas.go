package render

import (
	"fmt"
	"image"
	"image/draw"
)

// SpriteID identifies a pre-baked sprite in an Atlas — resolved by array
// index (Atlas.UV), never by map lookup, so it's safe on the draw hot path.
type SpriteID uint8

// SpriteDrawer paints one sprite's size x size pixels into dst — passed to
// Atlas.Register to bake a new sprite.
type SpriteDrawer func(dst *Canvas, size int)

// AtlasSource supplies the sprite sheet a Frame draws from: each sprite's rectangle on it, and a
// white texel plain colours sample, so they are drawn in the same call as the sprites.
type AtlasSource interface {
	Atlas() *Image
	UV(id SpriteID) (sx0, sy0, sx1, sy1 float32)
	White() (u, v float32)
}

// maxAtlasWidth is where Close starts a new row of sprites, so that a sheet of
// many or large sprites stays inside what a GPU accepts as one texture.
const maxAtlasWidth = 4096

// Atlas is an AtlasSource built by registering sprites, each at a size of its own, and
// then Close — which is when the sheet is laid out and baked, so nothing has to be sized up front.
type Atlas struct {
	slots  []slot // indexed by SpriteID
	image  *Image
	closed bool
	// white is the top-left of a 3x3 white patch after the sprites.
	whiteX, whiteY float32
}

// slot is one registered sprite: how to draw it and, after Close, where it sits on the sheet.
type slot struct {
	size           int
	draw           SpriteDrawer
	x0, y0, x1, y1 float32
}

var _ AtlasSource = (*Atlas)(nil)

// NewAtlas starts an empty atlas: Add its sprites, then Close it before the game loop starts.
func NewAtlas() *Atlas { return &Atlas{} }

// Add takes draw on as a size x size sprite in slot id — a kind's SpriteID, a slot issued by the
// world's kinds (NewSprite), a board's Covering — and hands it back as a Slot: chain Under for
// the looks the sprite takes under an effect. Panics after Close or if the slot is taken.
func (a *Atlas) Add(id SpriteID, size int, draw SpriteDrawer) Slot {
	if a.closed {
		panic("gram: Atlas.Add after Close")
	}
	if size <= 0 {
		panic(fmt.Sprintf("gram: Atlas sprite %d added with size %d", id, size))
	}
	for int(id) >= len(a.slots) {
		a.slots = append(a.slots, slot{})
	}
	if a.slots[id].draw != nil {
		panic(fmt.Sprintf("gram: Atlas sprite %d added twice", id))
	}
	a.slots[id] = slot{size: size, draw: draw}
	return Slot{a: a, id: id, size: size}
}

// Slot is one sprite added to an Atlas, what the looks under effects chain on.
type Slot struct {
	a    *Atlas
	id   SpriteID
	size int
}

// Dresser issues the atlas slot drawn in place of a sprite while a state holds: rule/effect's
// Effect is one, through its Look.
type Dresser interface{ Look(of SpriteID) SpriteID }

// Under adds the sprite's look under d, the same size — drawn in its place while the effect's
// marker is on: the witch gone white under frozen.
func (s Slot) Under(d Dresser, draw SpriteDrawer) Slot {
	s.a.Add(d.Look(s.id), s.size, draw)
	return s
}

// Close lays the registered sprites out on one sheet and bakes them; call once, after Register.
func (a *Atlas) Close() {
	if a.closed {
		return
	}
	a.closed = true

	width, height := a.layout()
	sheet := newCanvas(max(width, 1), max(height, 1))
	wx, wy := int(a.whiteX), int(a.whiteY)
	draw.Draw(sheet.RGBA, image.Rect(wx, wy, wx+3, wy+3), image.White, image.Point{}, draw.Src)
	for i := range a.slots {
		s := &a.slots[i]
		if s.draw == nil {
			continue
		}
		sprite := newCanvas(s.size, s.size)
		s.draw(sprite, s.size)
		draw.Draw(sheet.RGBA, image.Rect(int(s.x0), int(s.y0), int(s.x0)+s.size, int(s.y0)+s.size), sprite.RGBA, image.Point{}, draw.Over)
		s.draw = nil
	}
	a.image = NewImage(sheet.Bounds().Dx(), sheet.Bounds().Dy())
	a.image.WritePixels(sheet.Pix)
}

// layout shelves the sprites in slot order, then the white patch, and reports how large a sheet
// that takes.
func (a *Atlas) layout() (width, height int) {
	x, y, rowHeight := 0, 0, 0
	for i := range a.slots {
		s := &a.slots[i]
		if s.draw == nil {
			continue
		}
		if x > 0 && x+s.size > maxAtlasWidth {
			x, y, rowHeight = 0, y+rowHeight, 0
		}
		s.x0, s.y0 = float32(x), float32(y)
		s.x1, s.y1 = float32(x+s.size), float32(y+s.size)
		x += s.size
		rowHeight = max(rowHeight, s.size)
		width = max(width, x)
	}
	if x > 0 && x+3 > maxAtlasWidth {
		x, y, rowHeight = 0, y+rowHeight, 0
	}
	a.whiteX, a.whiteY = float32(x), float32(y)
	return max(width, x+3), y + max(rowHeight, 3)
}

func (a *Atlas) Atlas() *Image {
	if !a.closed {
		panic("gram: Atlas used before Close — its sheet does not exist yet")
	}
	return a.image
}

// White is the middle of the sheet's white patch.
func (a *Atlas) White() (u, v float32) {
	if !a.closed {
		panic("gram: Atlas used before Close — its sheet does not exist yet")
	}
	return a.whiteX + 1.5, a.whiteY + 1.5
}

func (a *Atlas) UV(id SpriteID) (sx0, sy0, sx1, sy1 float32) {
	if !a.closed {
		panic("gram: Atlas used before Close — its sheet does not exist yet")
	}
	if int(id) >= len(a.slots) || a.slots[id].size == 0 {
		panic(fmt.Sprintf("gram: Atlas has no sprite %d — it was never registered", id))
	}
	s := &a.slots[id]
	return s.x0, s.y0, s.x1, s.y1
}
