package world

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// spriteBatch is render.QuadBatch that draws a wrapped entity as the slices of its sprite
// on either side of the world edge.
type spriteBatch struct {
	batch  *render.QuadBatch
	camera camera.Camera
	worldW float32
	worldH float32
	quads  int // gathered since the last reset
}

func newSpriteBatch(atlas render.AtlasSource, worldW, worldH uint32) spriteBatch {
	return spriteBatch{
		batch:  render.NewQuadBatch(atlas),
		worldW: float32(worldW),
		worldH: float32(worldH),
	}
}

// reset empties the batch for a frame drawn through cam.
func (b *spriteBatch) reset(cam camera.Camera) { b.batch.Reset(cam); b.camera = cam; b.quads = 0 }

func (b *spriteBatch) drawQuad(pos Position, id render.SpriteID) {
	if !b.camera.Visible(pos.AABB.AABB) {
		return
	}
	sizeX := float32(pos.AABB.Size.X)
	sizeY := float32(pos.AABB.Size.Y)

	render.VisitWrapImages(pos.AABB, b.worldW, b.worldH, func(img geom.AABB, dx, dy float32) bool {
		tlx, tly := float32(img.TopLeft.X), float32(img.TopLeft.Y)
		brx, bry := float32(img.BottomRight.X), float32(img.BottomRight.Y)

		u0, u1 := uvSpan(brx-tlx, sizeX, dx)
		v0, v1 := uvSpan(bry-tly, sizeY, dy)
		b.batch.AppendQuadUV(tlx, tly, brx, bry, id, u0, v0, u1, v1)
		b.quads++
		return true
	})
}

// uvSpan is the slice of the sprite one image shows along one axis.
func uvSpan(imgSize, spriteSize, shift float32) (float32, float32) {
	visible := imgSize / spriteSize
	if shift == 0 {
		return 0, visible
	}
	return 1 - visible, 1
}

// flush draws the gathered quads; a nil screen draws nothing.
func (b *spriteBatch) flush(screen *ebiten.Image) {
	if screen == nil {
		return
	}
	b.batch.Flush(screen)
}
