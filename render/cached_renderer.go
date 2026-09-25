package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
)

// CachedRenderer draws inner once into an offscreen image and reuses it every
// frame — call Invalidate to force a redraw on the next Draw. The image follows the screen's size:
// a resized window gets a fresh one, drawn anew.
type CachedRenderer struct {
	inner Renderer
	image *ebiten.Image
	dirty bool
}

// NewCachedRenderer wraps inner, caching its output at w×h until invalidated or the screen changes size.
func NewCachedRenderer(inner Renderer, w, h int) *CachedRenderer {
	return &CachedRenderer{inner: inner, image: ebiten.NewImage(w, h), dirty: true}
}

func (c *CachedRenderer) Init(si *goke.SysInit) { c.inner.Init(si) }

// Invalidate forces the next Draw to redraw the cached image.
func (c *CachedRenderer) Invalidate() { c.dirty = true }

func (c *CachedRenderer) Draw(screen *ebiten.Image) {
	if b, have := screen.Bounds(), c.image.Bounds(); b.Dx() != have.Dx() || b.Dy() != have.Dy() {
		c.image.Deallocate()
		c.image, c.dirty = ebiten.NewImage(b.Dx(), b.Dy()), true
	}
	if c.dirty {
		c.image.Clear()
		c.inner.Draw(c.image)
		c.dirty = false
	}
	screen.DrawImage(c.image, nil)
}
