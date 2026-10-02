// Package look is the world's own look: seen from above, each entity's sprite over its box.
package look

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/render"
)

// Flat is the world seen from above: a sprite over its box, in a piece per image where the box
// crosses a wrap seam; it knows no wind, so nothing sways. Readied for a frame (Begin) it gathers
// the sprites and draws them on the GPU (DrawSprites); else it lays them on the frame.
type Flat struct {
	worldW, worldH float32
	quads          []camera.Quad
	sprites        render.Sprites
	direct         bool // readied for this frame
}

// NewFlat is the flat look over a world width x height.
func NewFlat(width, height float32) *Flat { return &Flat{worldW: width, worldH: height} }

// Begin readies the look to gather the frame's sprites for the GPU.
func (l *Flat) Begin(camera.Camera) {
	l.sprites.Reset()
	l.direct = true
}

// DrawSprites draws the sprites gathered since Begin.
func (l *Flat) DrawSprites(t render.Target, cam camera.Camera, _ render.Uniforms) {
	if l.direct {
		l.sprites.Draw(t, cam)
	}
	l.direct = false
}

func (l *Flat) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, _ entity.Z, atlas render.AtlasSource, id render.SpriteID, light render.Light, _ float32) {
	sizeX, sizeY := float32(box.Size.X), float32(box.Size.Y)
	render.VisitWrapImages(box, l.worldW, l.worldH, func(img geom.AABB, dx, dy float32) bool {
		x0, y0 := float32(img.TopLeft.X), float32(img.TopLeft.Y)
		x1, y1 := float32(img.BottomRight.X), float32(img.BottomRight.Y)
		u0, u1 := uvSpan(x1-x0, sizeX, dx)
		v0, v1 := uvSpan(y1-y0, sizeY, dy)
		if l.direct {
			l.sprites.Rect(cam, atlas, id, x0, y0, x1, y1, u0, v0, u1, v1, light)
		} else {
			f.SpriteRectUV(render.Objects, 0, atlas, id, x0, y0, x1, y1, u0, v0, u1, v1, render.Lit(light))
		}
		return true
	})
}

func (*Flat) Drawn(cam camera.Camera, box geom.AABB, z entity.Z) render.Corners {
	return render.ProjectCorners(cam, float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), float32(z.Altitude))
}

func (l *Flat) Footprint(cam camera.Camera, box geom.AABB, _ float32, dst []render.Corners) []render.Corners {
	l.quads = cam.ToScreenQuads(float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), l.quads[:0])
	for _, q := range l.quads {
		dst = append(dst, render.Corners{{q.X0, q.Y0}, {q.X1, q.Y0}, {q.X0, q.Y1}, {q.X1, q.Y1}})
	}
	return dst
}

// uvSpan is the slice of the sprite one image shows along one axis.
func uvSpan(imgSize, spriteSize, shift float32) (float32, float32) {
	visible := imgSize / spriteSize
	if shift == 0 {
		return 0, visible
	}
	return 1 - visible, 1
}
