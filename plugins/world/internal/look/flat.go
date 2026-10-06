// Package look is the world's own look: seen from above, each entity's sprite over its box.
package look

import (
	"math"

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

func (l *Flat) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, _ entity.Z, atlas render.AtlasSource, a render.Appearance, light render.Light) {
	if a.Angle != 0 {
		l.turned(f, cam, box, atlas, a, light)
		return
	}
	id := a.SpriteID
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

// turned draws the sprite whole, turned about the middle of its box — one piece per wrap image,
// never split across a seam.
func (l *Flat) turned(f *render.Frame, cam camera.Camera, box plane.AABB, atlas render.AtlasSource, a render.Appearance, light render.Light) {
	rad := a.Angle * math.Pi / 180
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	if l.direct {
		l.sprites.Turned(cam, atlas, a.SpriteID, x0, y0, x1, y1, rad, light)
		return
	}
	sx0, sy0, sx1, sy1 := atlas.UV(a.SpriteID)
	sin, cos := float32(math.Sin(float64(rad))), float32(math.Cos(float64(rad)))
	l.quads = cam.ToScreenQuads(x0, y0, x1, y1, l.quads[:0])
	for _, q := range l.quads {
		cx, cy := (q.X0+q.X1)/2, (q.Y0+q.Y1)/2
		turn := func(x, y float32) [2]float32 {
			ox, oy := x-cx, y-cy
			return [2]float32{cx + ox*cos + oy*sin, cy + oy*cos - ox*sin}
		}
		dst := render.Corners{turn(q.X0, q.Y0), turn(q.X1, q.Y0), turn(q.X0, q.Y1), turn(q.X1, q.Y1)}
		f.SpritePart(render.Objects, 0, atlas, [4]float32{sx0, sy0, sx1, sy1}, dst, render.Lit(light))
	}
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
