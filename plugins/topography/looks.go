package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/topography/heightfield"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// inRelief reports whether cam looks at the world in relief — isometrically or in perspective: a
// camera of the topography's not looking from above.
func inRelief(cam camera.Camera) bool {
	c, ok := cam.(*viewCamera)
	return ok && c.relief()
}

var _ board.Look = boardLook{}

// boardLook lays the board's cells as the camera looks: blocks in the isometric view, flat tiles
// from above.
type boardLook struct{ d *dresser }

func (l boardLook) Cell(f *render.Frame, cam camera.Camera, t *board.Tile) {
	if inRelief(cam) {
		blocks{d: l.d}.Cell(f, cam, t)
		return
	}
	board.FlatLook().Cell(f, cam, t)
}

var _ world.Look = worldLook{}

// worldLook lays the world's entities as the camera looks: billboards standing upright in the
// isometric view, the world's own flat sprites from above — lit by the sky's sun on level ground,
// leaning with its wind, each casting its shadow on the relief away from the sun.
type worldLook struct {
	flat  world.Look
	d     *dresser
	field *heightfield.Renderer // the ground drawn from its heightmap, hiding what lies behind it; nil none
}

func (l worldLook) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource, id render.SpriteID, light render.Light, sway float32) {
	if l.field != nil && l.field.Hides(cam, float32(box.TopLeft.X+box.Size.X/2), float32(box.TopLeft.Y+box.Size.Y/2), float32(z.Altitude+z.Height/2)) {
		return // behind the ground the heightfield draws, which no painter's order hides it by
	}
	sun, weather := l.d.sky.Sun(), l.d.sky.Air()
	if l.d.heights {
		sun.Shadow(f, cam, box.AABB, z, l.groundAt)
	}
	if inRelief(cam) {
		billboards{d: l.d}.Sprite(f, cam, box, z, atlas, id, light, sway)
		return
	}
	if sway > 0 { // seen from above by its top, as high as it is wide, leaning with the wind
		sizeX, sizeY := float32(box.Size.X), float32(box.Size.Y)
		cx, cy := float32(box.TopLeft.X)+sizeX/2, float32(box.TopLeft.Y)+sizeY/2
		lx, ly := weather.Sway(f.Time(), cx, cy, sway)
		rise := max(sizeX, sizeY)
		box = plane.NewAABB(geom.NewVec(box.TopLeft.X+float64(lx*rise), box.TopLeft.Y+float64(ly*rise)), box.Size.X, box.Size.Y)
	}
	l.flat.Sprite(f, cam, box, z, atlas, id, lit(sun, light), 0)
}

// groundAt is the relief's height at a point, for the shadows.
func (l worldLook) groundAt(x, y float32) float32 {
	return float32(l.d.relief.GroundAt(geom.NewVec(float64(x), float64(y))))
}

func (l worldLook) Drawn(cam camera.Camera, box geom.AABB, z world.Z) render.Corners {
	if inRelief(cam) {
		return billboards{d: l.d}.Drawn(cam, box, z)
	}
	return l.flat.Drawn(cam, box, z)
}

func (l worldLook) Footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners {
	if inRelief(cam) {
		return billboards{d: l.d}.Footprint(cam, box, alt, dst)
	}
	return l.flat.Footprint(cam, box, alt, dst)
}
