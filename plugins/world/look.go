package world

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// Look is how what stands in the world lies on the screen through a camera: the world's way of
// drawing its entities, which the renderer, picking and outlines ask. The world starts with a flat
// look, seen from above; a view plugin puts its own in with Plugin.SetLook.
type Look interface {
	// Sprite hands f sprite id of atlas for an entity whose box stands at altitude alt, on the
	// render.Objects tier, in light, swaying in the frame's wind as much as sway says
	// (Appearance.Sway).
	Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, alt float32, atlas render.AtlasSource, id render.SpriteID, light render.Light, sway float32)
	// Drawn is the screen quad that sprite covers, for picking.
	Drawn(cam camera.Camera, box geom.AABB, alt float32) render.Corners
	// Footprint appends to dst the ground under box on screen, in pieces where it crosses a wrap
	// seam, for outlines.
	Footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners
}

// Cameras makes a camera over a width x height world with the given edges, configured by cfg; a
// view plugin puts its own in with Plugin.SetCameras.
type Cameras func(width, height uint32, edges aabbworld.Edges, cfg camera.Config) camera.Camera

// flatLook is the world seen from above: a sprite over its box, in a piece per image where the
// box crosses a wrap seam.
type flatLook struct {
	worldW, worldH float32
	quads          []camera.Quad
}

func (l *flatLook) Sprite(f *render.Frame, _ camera.Camera, box plane.AABB, _ float32, atlas render.AtlasSource, id render.SpriteID, light render.Light, sway float32) {
	sizeX, sizeY := float32(box.Size.X), float32(box.Size.Y)
	if sway > 0 { // seen from above by its top, as high as it is wide, leaning with the wind
		cx, cy := float32(box.TopLeft.X)+sizeX/2, float32(box.TopLeft.Y)+sizeY/2
		lx, ly := render.Sway(f.Time(), f.Wind(), cx, cy, sway)
		rise := max(sizeX, sizeY)
		box = plane.NewAABB(geom.NewVec(box.TopLeft.X+float64(lx*rise), box.TopLeft.Y+float64(ly*rise)), box.Size.X, box.Size.Y)
	}
	render.VisitWrapImages(box, l.worldW, l.worldH, func(img geom.AABB, dx, dy float32) bool {
		x0, y0 := float32(img.TopLeft.X), float32(img.TopLeft.Y)
		x1, y1 := float32(img.BottomRight.X), float32(img.BottomRight.Y)
		u0, u1 := uvSpan(x1-x0, sizeX, dx)
		v0, v1 := uvSpan(y1-y0, sizeY, dy)
		f.SpriteRectUV(render.Objects, 0, atlas, id, x0, y0, x1, y1, u0, v0, u1, v1, render.Lit(light))
		return true
	})
}

func (*flatLook) Drawn(cam camera.Camera, box geom.AABB, alt float32) render.Corners {
	return render.ProjectCorners(cam, float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), alt)
}

func (l *flatLook) Footprint(cam camera.Camera, box geom.AABB, _ float32, dst []render.Corners) []render.Corners {
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
