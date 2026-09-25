package selection

import (
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*FollowSystem)(nil)

// FollowSystem keeps a camera on the entity tagged Followed: Follow tags the one Selected unit or
// untags the followed one, and every tick the camera of whoever asked is centred on it at its
// altitude. A player who
// moves the camera by hand ends the following; zooming does not.
type FollowSystem struct {
	follows *control.Queue[Follow]
	camera  camera.Camera
	tags    Tags

	query *goke.Query
	base  goke.Comp[world.Base]
	marks goke.Comp[plugin.Tags[Family]]
	z     goke.OptComp[world.Z]

	// Where the followed point was drawn right after the last centring, at which zoom: a camera
	// moved by hand no longer draws it there.
	centred          bool
	pointX, pointY   float64
	pointZ           float64
	screenX, screenY float32
	zoom             float32
}

// NewFollowSystem builds a FollowSystem draining follows and moving the camera of whoever asked.
func NewFollowSystem(follows *control.Queue[Follow], tags Tags) *FollowSystem {
	return &FollowSystem{follows: follows, tags: tags}
}

func (s *FollowSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base, &s.marks).Optional(&s.z).Build()
}

func (s *FollowSystem) Update(*goke.CmdBuf, time.Duration) {
	s.follows.Drain(func(i control.Issued[Follow]) {
		if i.Command.Camera != nil {
			s.toggle(i.Command.Camera)
		}
	})
	if s.camera == nil {
		return
	}

	if s.centred && s.movedByHand() {
		s.untagAll()
	}
	s.centred = false
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		bases, marks, zs := s.base.Slice(cursor), s.marks.Slice(cursor), s.z.Slice(cursor)
		for i := range cursor.IDs {
			if !marks[i].Has(s.tags.Followed) {
				continue
			}
			box := bases[i].Pos.AABB
			s.pointX = (box.TopLeft.X + box.BottomRight.X) / 2
			s.pointY = (box.TopLeft.Y + box.BottomRight.Y) / 2
			s.pointZ = 0
			if zs != nil {
				s.pointZ = zs[i].Altitude
			}
			s.camera.CenterOn(s.pointX, s.pointY, s.pointZ)
			s.screenX, s.screenY = s.camera.Project(float32(s.pointX), float32(s.pointY), float32(s.pointZ))
			s.zoom, s.centred = s.camera.Zoom(), true
			return
		}
	}
}

// movedByHand reports whether, at an unchanged zoom, the point centred last tick is drawn elsewhere now.
func (s *FollowSystem) movedByHand() bool {
	if s.camera.Zoom() != s.zoom {
		return false
	}
	sx, sy := s.camera.Project(float32(s.pointX), float32(s.pointY), float32(s.pointZ))
	return math.Abs(float64(sx-s.screenX)) > 0.5 || math.Abs(float64(sy-s.screenY)) > 0.5
}

// toggle stops following when something is followed, else follows the one Selected unit; with
// none or several selected it does nothing.
func (s *FollowSystem) toggle(cam camera.Camera) {
	if s.untagAll() {
		return
	}
	s.camera = cam
	var one uid.UID64
	selected := 0
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		for i, m := range s.marks.Slice(cur) {
			if m.Has(s.tags.Selectable) && m.Has(s.tags.Selected) {
				selected, one = selected+1, cur.IDs[i]
			}
		}
	}
	if selected != 1 {
		return
	}
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		for i, id := range cur.IDs {
			if id == one {
				s.marks.Slice(cur)[i] = s.marks.Slice(cur)[i].With(s.tags.Followed)
			}
		}
	}
}

// untagAll takes Followed off every entity and reports whether any carried it.
func (s *FollowSystem) untagAll() bool {
	any := false
	s.query.All()
	for s.query.Next() {
		marks := s.marks.Slice(s.query.Cursor())
		for i := range marks {
			if marks[i].Has(s.tags.Followed) {
				marks[i], any = marks[i].Without(s.tags.Followed), true
			}
		}
	}
	s.centred = false
	return any
}
