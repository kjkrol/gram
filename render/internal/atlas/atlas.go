package atlas

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/kjkrol/gram/render/internal/gfx"
)

// SpriteDrawer paints one sprite's size x size pixels into dst.
type SpriteDrawer func(dst *Canvas, size int)

// what a GPU accepts as one texture
const maxAtlasWidth = 4096

// Atlas is an AtlasSource built by adding sprites, each at a size of its own, and then Close,
// which lays the sheet out and bakes it.
type Atlas struct {
	slots  []slot // indexed by SpriteID
	image  *gfx.Image
	closed bool
	// the top-left of a 3x3 white patch after the sprites
	whiteX, whiteY float32
}

type slot struct {
	size           int
	draw           SpriteDrawer
	x0, y0, x1, y1 float32
}

var _ gfx.AtlasSource = (*Atlas)(nil)

// NewAtlas starts an empty atlas.
func NewAtlas() *Atlas { return &Atlas{} }

// Sprited is whoever is drawn from one slot of the sheet: a kind's handle, or a bare SpriteID,
// which stands for itself.
type Sprited interface{ SpriteID() gfx.SpriteID }

// Add takes draw on as a size x size sprite in of's slot. It panics after Close or if the slot
// is taken.
func (a *Atlas) Add(of Sprited, size int, draw SpriteDrawer) {
	id := of.SpriteID()
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
}

// Close lays the sprites added out on one sheet and bakes them.
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
	a.image = gfx.NewImage(sheet.Bounds().Dx(), sheet.Bounds().Dy())
	a.image.WritePixels(sheet.Pix)
}

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

func (a *Atlas) Atlas() *gfx.Image {
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

func (a *Atlas) UV(id gfx.SpriteID) (sx0, sy0, sx1, sy1 float32) {
	if !a.closed {
		panic("gram: Atlas used before Close — its sheet does not exist yet")
	}
	if int(id) >= len(a.slots) || a.slots[id].size == 0 {
		panic(fmt.Sprintf("gram: Atlas has no sprite %d — it was never registered", id))
	}
	s := &a.slots[id]
	return s.x0, s.y0, s.x1, s.y1
}
