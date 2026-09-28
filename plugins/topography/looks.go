package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
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
// isometric view, the world's own flat sprites from above.
type worldLook struct{ flat world.Look }

func (l worldLook) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource, id render.SpriteID, light render.Light, sway float32) {
	if inRelief(cam) {
		billboards{}.Sprite(f, cam, box, z, atlas, id, light, sway)
		return
	}
	l.flat.Sprite(f, cam, box, z, atlas, id, light, sway)
}

func (l worldLook) Drawn(cam camera.Camera, box geom.AABB, z world.Z) render.Corners {
	if inRelief(cam) {
		return billboards{}.Drawn(cam, box, z)
	}
	return l.flat.Drawn(cam, box, z)
}

func (l worldLook) Footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners {
	if inRelief(cam) {
		return billboards{}.Footprint(cam, box, alt, dst)
	}
	return l.flat.Footprint(cam, box, alt, dst)
}
