package selection

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*SelectionSystem)(nil)

// pickReach is how far round a Select's Box, in world units, the entities drawn into its Screen
// rectangle may stand: a unit high up is drawn above its ground.
const pickReach = 160

// SelectionSystem carries out Select commands as the Selected tag on Selectable entities — a bit
// flipped in place, seen the same tick. A Select with a Screen rectangle hits the entities drawn
// into it, as the camera draws them: their box on the ground, or through an isometric camera the
// billboard standing on their centre at their altitude.
type SelectionSystem struct {
	selects *control.Inbox[Select]
	space   *aabbworld.Space
	camera  camera.Camera
	tags    Tags

	query *goke.Query
	marks goke.Comp[plugin.Tags[Family]]

	lookup     *goke.Query
	lookupBase goke.Comp[world.Base]
	lookupZ    goke.OptComp[world.Z]
}

// NewSelectionSystem builds a SelectionSystem draining selects over space, picking through cam.
func NewSelectionSystem(selects *control.Inbox[Select], space *aabbworld.Space, cam camera.Camera, tags Tags) *SelectionSystem {
	return &SelectionSystem{selects: selects, space: space, camera: cam, tags: tags}
}

func (s *SelectionSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.marks).Build()
	s.lookup = si.NewQueryBuilder(&s.lookupBase).Optional(&s.lookupZ).Build()
}

func (s *SelectionSystem) Update(_ *goke.CmdBuf, _ time.Duration) {
	s.selects.Drain(func(i control.Issued[Select]) {
		cmd := i.Command
		hit := make(map[uid.UID64]struct{}, len(cmd.IDs))
		switch {
		case cmd.IDs != nil:
			for _, id := range cmd.IDs {
				hit[id] = struct{}{}
			}
		case cmd.Screen == (geom.AABB{}):
			s.space.Query(cmd.Box, aabbworld.AnyCapability, func(id uid.UID64) { hit[id] = struct{}{} })
		default:
			s.space.Query(grow(cmd.Box, pickReach), aabbworld.AnyCapability, func(id uid.UID64) {
				if s.drawnIn(id, cmd.Screen) {
					hit[id] = struct{}{}
				}
			})
		}
		s.applySelection(hit, cmd.Additive)
	})
}

// drawnIn reports whether id is drawn into the screen rectangle: through an isometric camera as a
// billboard on its centre at its altitude, otherwise as its box.
func (s *SelectionSystem) drawnIn(id uid.UID64, screen geom.AABB) bool {
	if !s.lookup.Seek(id) {
		return false
	}
	cur := s.lookup.Cursor()
	box := s.lookupBase.At(cur).Pos.AABB
	alt := float32(0)
	if z := s.lookupZ.At(cur); z != nil {
		alt = float32(z.Altitude)
	}
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	var c render.Corners
	if _, iso := s.camera.Projection().(camera.Isometric); iso {
		c = render.Billboard(s.camera, (x0+x1)/2, (y0+y1)/2, alt, x1-x0, y1-y0)
	} else {
		c = render.ProjectCorners(s.camera, x0, y0, x1, y1, alt)
	}
	minX, minY, maxX, maxY := c[0][0], c[0][1], c[0][0], c[0][1]
	for _, p := range c[1:] {
		minX, maxX = min(minX, p[0]), max(maxX, p[0])
		minY, maxY = min(minY, p[1]), max(maxY, p[1])
	}
	return float64(maxX) >= screen.TopLeft.X && float64(minX) <= screen.BottomRight.X &&
		float64(maxY) >= screen.TopLeft.Y && float64(minY) <= screen.BottomRight.Y
}

// grow is box widened by reach on every side.
func grow(box geom.AABB, reach float64) geom.AABB {
	return geom.AABB{
		TopLeft:     geom.NewVec(box.TopLeft.X-reach, box.TopLeft.Y-reach),
		BottomRight: geom.NewVec(box.BottomRight.X+reach, box.BottomRight.Y+reach),
	}
}

// applySelection tags every hit Selectable entity Selected and, unless additive, untags the rest.
func (s *SelectionSystem) applySelection(hit map[uid.UID64]struct{}, additive bool) {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		marks := s.marks.Slice(cursor)
		for i, id := range cursor.IDs {
			if !marks[i].Has(s.tags.Selectable) {
				continue
			}
			if _, ok := hit[id]; ok {
				marks[i] = marks[i].With(s.tags.Selected)
			} else if !additive {
				marks[i] = marks[i].Without(s.tags.Selected)
			}
		}
	}
}
