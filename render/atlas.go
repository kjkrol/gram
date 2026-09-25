package render

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// SpriteID identifies a pre-baked sprite in an Atlas — resolved by array
// index (Atlas.UV), never by map lookup, so it's safe on the draw hot path.
type SpriteID uint8

// SpriteDrawer paints one sprite's size x size pixels into dst — passed to
// Atlas.Register to bake a new sprite.
type SpriteDrawer func(dst *ebiten.Image, size int)

// AtlasSource supplies the sprite sheet a Frame draws from: each sprite's rectangle on it, and a
// white texel plain colours sample, so they are drawn in the same call as the sprites.
type AtlasSource interface {
	Atlas() *ebiten.Image
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
	image  *ebiten.Image
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

// NewAtlas starts an empty atlas: Register its sprites, then Close it before the game loop starts.
func NewAtlas() *Atlas { return &Atlas{} }

// Register takes draw on as a size x size sprite and returns its SpriteID; panics after Close.
func (a *Atlas) Register(size int, draw SpriteDrawer) SpriteID {
	id := SpriteID(len(a.slots))
	a.RegisterAt(id, size, draw)
	return id
}

// RegisterAt takes draw on as a size x size sprite in slot id; panics after Close or if taken.
func (a *Atlas) RegisterAt(id SpriteID, size int, draw SpriteDrawer) {
	if a.closed {
		panic("gram: Atlas.Register after Close")
	}
	if size <= 0 {
		panic(fmt.Sprintf("gram: Atlas sprite %d registered with size %d", id, size))
	}
	for int(id) >= len(a.slots) {
		a.slots = append(a.slots, slot{})
	}
	if a.slots[id].draw != nil {
		panic(fmt.Sprintf("gram: Atlas sprite %d registered twice", id))
	}
	a.slots[id] = slot{size: size, draw: draw}
}

// Close lays the registered sprites out on one sheet and bakes them; call once, after Register.
func (a *Atlas) Close() {
	if a.closed {
		return
	}
	a.closed = true

	width, height := a.layout()
	a.image = ebiten.NewImage(max(width, 1), max(height, 1))
	a.image.SubImage(image.Rect(int(a.whiteX), int(a.whiteY), int(a.whiteX)+3, int(a.whiteY)+3)).(*ebiten.Image).Fill(color.White)
	for i := range a.slots {
		s := &a.slots[i]
		if s.draw == nil {
			continue
		}
		sprite := ebiten.NewImage(s.size, s.size)
		s.draw(sprite, s.size)
		opts := &ebiten.DrawImageOptions{}
		opts.GeoM.Translate(float64(s.x0), float64(s.y0))
		a.image.DrawImage(sprite, opts)
		s.draw = nil
	}
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

func (a *Atlas) Atlas() *ebiten.Image {
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
