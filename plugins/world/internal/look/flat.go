// Package look is the world's own look: seen from above, each entity's sprite over its box.
package look

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/internal/f32"
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

// FigureLift is how far over its height a figure lies among what is drawn back to front: over
// whatever a plugin lays on the ground beneath it.
const FigureLift = 2

// NewFlat is the flat look over a world width x height, wrapping across where wrapX says and
// down where wrapY does.
func NewFlat(width, height float32, wrapX, wrapY bool) *Flat {
	l := &Flat{worldW: width, worldH: height}
	l.sprites.Wrap(width, height, wrapX, wrapY)
	return l
}

// Fog has the sprites laid next turn into the fog by amount.
func (l *Flat) Fog(amount float32) { l.sprites.Fog(amount) }

// Begin readies the look to gather the frame's sprites for the GPU.
func (l *Flat) Begin(*render.Frame, camera.Camera) {
	l.sprites.Reset()
	l.direct = true
}

// DrawSprites draws the sprites gathered since Begin, as often as a frame laid in layers asks.
func (l *Flat) DrawSprites(t render.Target, cam camera.Camera, u render.Uniforms) {
	if l.direct {
		l.sprites.Draw(t, cam, u)
	}
}

func (l *Flat) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z entity.Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	stands := at(f, box, z.Top())
	l.sprites.Layer(stands, false)
	l.sprites.Take(lit.Takes)
	l.sprites.Direct(lit.Direct)
	l.lay(f, cam, stands, box, atlas, a, lit.Light)
}

// at is where a figure over box lies at height z, among what is drawn back to front
func at(f *render.Frame, box plane.AABB, z float32) render.At {
	return f.Lifted(box.TopLeft.X+box.Size.X/2, box.TopLeft.Y+box.Size.Y/2,
		z, FigureLift)
}

// Lie lays the sprite as Sprite does, at its Altitude: under whatever stands higher.
func (l *Flat) Lie(f *render.Frame, cam camera.Camera, box plane.AABB, z entity.Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	lies := at(f, box, z.Altitude)
	l.sprites.Layer(lies, true)
	l.sprites.Take(lit.Takes)
	l.sprites.Direct(lit.Direct)
	l.lay(f, cam, lies, box, atlas, a, lit.Light)
}

// Lay lays the sprite over the rectangle lie says and, lie being Edged, the edge sprite along its
// long sides as strips a tenth of its Width wide.
func (l *Flat) Lay(f *render.Frame, cam camera.Camera, box plane.AABB, dir geom.Vec,
	lie entity.Lie, z entity.Z, atlas render.AtlasSource, a render.Appearance,
	edge render.SpriteID, lit render.Illumination) {
	lies := at(f, box, z.Altitude)
	l.sprites.Layer(lies, true)
	l.sprites.Take(lit.Takes)
	l.sprites.Direct(lit.Direct)
	light := lit.Light
	if dir == (geom.Vec{}) {
		dir = geom.NewVec(1, 0)
	}
	a.Angle = f32.Atan2(-dir.Y, dir.X) * 180 / math.Pi
	m := geom.NewVec(box.TopLeft.X+box.Size.X/2+dir.X*lie.Ahead,
		box.TopLeft.Y+box.Size.Y/2+dir.Y*lie.Ahead)
	l.turned(f, cam, lies, around(m, lie.Length, lie.Width), atlas, a, light)
	if !lie.Edged {
		return
	}
	a.SpriteID = edge
	for _, side := range [2]float32{-1, 1} {
		at := geom.NewVec(m.X-dir.Y*side*lie.Width/2, m.Y+dir.X*side*lie.Width/2)
		l.turned(f, cam, lies, around(at, lie.Length, lie.Width/10), atlas, a, light)
	}
}

func around(m geom.Vec, w, h float32) plane.AABB {
	return plane.NewAABB(geom.NewVec(m.X-w/2, m.Y-h/2), w, h)
}

func (l *Flat) lay(f *render.Frame, cam camera.Camera, at render.At, box plane.AABB,
	atlas render.AtlasSource, a render.Appearance, light render.Light) {
	if a.Angle != 0 || !l.direct && !cam.Projection().Wraps() {
		l.turned(f, cam, at, box, atlas, a, light)
		return
	}
	id := a.SpriteID
	sizeX, sizeY := box.Size.X, box.Size.Y
	render.VisitWrapImages(box, l.worldW, l.worldH, func(img geom.AABB, dx, dy float32) bool {
		x0, y0 := img.TopLeft.X, img.TopLeft.Y
		x1, y1 := img.BottomRight.X, img.BottomRight.Y
		u0, u1 := uvSpan(x1-x0, sizeX, dx)
		v0, v1 := uvSpan(y1-y0, sizeY, dy)
		if l.direct {
			l.sprites.Rect(atlas, id, x0, y0, x1, y1, u0, v0, u1, v1, light)
		} else {
			f.SpriteRectUV(at, atlas, id, x0, y0, x1, y1, u0, v0, u1, v1, render.Lit(light))
		}
		return true
	})
}

// one piece per wrap image, never split across a seam
func (l *Flat) turned(f *render.Frame, cam camera.Camera, at render.At, box plane.AABB,
	atlas render.AtlasSource, a render.Appearance, light render.Light) {
	rad := a.Angle * math.Pi / 180
	x0, y0 := box.TopLeft.X, box.TopLeft.Y
	x1, y1 := box.BottomRight.X, box.BottomRight.Y
	if l.direct {
		l.sprites.Turned(atlas, a.SpriteID, x0, y0, x1, y1, rad, light)
		return
	}
	sx0, sy0, sx1, sy1 := atlas.UV(a.SpriteID)
	sin, cos := f32.Sin(rad), f32.Cos(rad)
	if !cam.Projection().Wraps() { // turned in the world, as the view may be
		cx, cy := (x0+x1)/2, (y0+y1)/2
		var dst render.Corners
		for i, c := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
			ox, oy := c[0]-cx, c[1]-cy
			dst[i][0], dst[i][1] = cam.Project(cx+ox*cos+oy*sin, cy+oy*cos-ox*sin, 0)
		}
		f.SpritePart(at, atlas, [4]float32{sx0, sy0, sx1, sy1}, dst, render.Lit(light))
		return
	}
	l.quads = cam.ToScreenQuads(x0, y0, x1, y1, l.quads[:0])
	for _, q := range l.quads {
		cx, cy := (q.X0+q.X1)/2, (q.Y0+q.Y1)/2
		turn := func(x, y float32) [2]float32 {
			ox, oy := x-cx, y-cy
			return [2]float32{cx + ox*cos + oy*sin, cy + oy*cos - ox*sin}
		}
		dst := render.Corners{turn(q.X0, q.Y0), turn(q.X1, q.Y0), turn(q.X0, q.Y1), turn(q.X1, q.Y1)}
		f.SpritePart(at, atlas, [4]float32{sx0, sy0, sx1, sy1}, dst, render.Lit(light))
	}
}

func (*Flat) Drawn(cam camera.Camera, box geom.AABB, z entity.Z) render.Corners {
	return render.ProjectCorners(cam,
		box.TopLeft.X,
		box.TopLeft.Y,
		box.BottomRight.X,
		box.BottomRight.Y,
		z.Altitude,
	)
}

// A piece a copy of the box a camera over a wrapping world shows; through any other its corners
// where the camera lays them, turning with a view turned.
func (l *Flat) Footprint(
	cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners,
) []render.Corners {
	x0, y0 := box.TopLeft.X, box.TopLeft.Y
	x1, y1 := box.BottomRight.X, box.BottomRight.Y
	if !cam.Projection().Wraps() {
		return append(dst, render.ProjectCorners(cam, x0, y0, x1, y1, alt))
	}
	l.quads = cam.ToScreenQuads(x0, y0, x1, y1, l.quads[:0])
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
