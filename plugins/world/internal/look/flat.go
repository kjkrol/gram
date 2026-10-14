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

// Flat is the world seen from above: a sprite over its box, split where it crosses a wrap seam.
// Readied for a frame it draws the sprites on the GPU by height, a standing one at its top and a
// lying one at its Altitude before those standing as high, else it lays them on the frame.
type Flat struct {
	worldW, worldH float32
	quads          []camera.Quad
	sprites        render.Sprites
	direct         bool
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

func (l *Flat) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z entity.Z,
	atlas render.AtlasSource, a render.Appearance, light render.Light) {
	l.sprites.Layer(float32(z.Top()), false)
	l.lay(f, cam, render.Objects, box, atlas, a, light)
}

// Lie lays the sprite as Sprite does, at its Altitude: under whatever stands higher.
func (l *Flat) Lie(f *render.Frame, cam camera.Camera, box plane.AABB, z entity.Z,
	atlas render.AtlasSource, a render.Appearance, light render.Light) {
	l.sprites.Layer(float32(z.Altitude), true)
	l.lay(f, cam, render.Objects-1, box, atlas, a, light)
}

// Lay lays the sprite over the rectangle lie says and, lie being Edged, the edge sprite along its
// long sides as strips a tenth of its Width wide.
func (l *Flat) Lay(f *render.Frame, cam camera.Camera, box plane.AABB, dir geom.Vec,
	lie entity.Lie, z entity.Z, atlas render.AtlasSource, a render.Appearance,
	edge render.SpriteID, light render.Light) {
	l.sprites.Layer(float32(z.Altitude), true)
	if dir == (geom.Vec{}) {
		dir = geom.NewVec(1, 0)
	}
	a.Angle = float32(math.Atan2(-dir.Y, dir.X) * 180 / math.Pi)
	m := geom.NewVec(box.TopLeft.X+box.Size.X/2+dir.X*lie.Ahead,
		box.TopLeft.Y+box.Size.Y/2+dir.Y*lie.Ahead)
	l.turned(f, cam, render.Objects-1, around(m, lie.Length, lie.Width), atlas, a, light)
	if !lie.Edged {
		return
	}
	a.SpriteID = edge
	for _, side := range [2]float64{-1, 1} {
		at := geom.NewVec(m.X-dir.Y*side*lie.Width/2, m.Y+dir.X*side*lie.Width/2)
		l.turned(f, cam, render.Objects-1, around(at, lie.Length, lie.Width/10), atlas, a, light)
	}
}

func around(m geom.Vec, w, h float64) plane.AABB {
	return plane.NewAABB(geom.NewVec(m.X-w/2, m.Y-h/2), w, h)
}

func (l *Flat) lay(f *render.Frame, cam camera.Camera, tier render.Tier, box plane.AABB,
	atlas render.AtlasSource, a render.Appearance, light render.Light) {
	if a.Angle != 0 {
		l.turned(f, cam, tier, box, atlas, a, light)
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
			f.SpriteRectUV(tier, 0, atlas, id, x0, y0, x1, y1, u0, v0, u1, v1, render.Lit(light))
		}
		return true
	})
}

// one piece per wrap image, never split across a seam
func (l *Flat) turned(f *render.Frame, cam camera.Camera, tier render.Tier, box plane.AABB,
	atlas render.AtlasSource, a render.Appearance, light render.Light) {
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
		f.SpritePart(tier, 0, atlas, [4]float32{sx0, sy0, sx1, sy1}, dst, render.Lit(light))
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

func uvSpan(imgSize, spriteSize, shift float32) (float32, float32) {
	visible := imgSize / spriteSize
	if shift == 0 {
		return 0, visible
	}
	return 1 - visible, 1
}
