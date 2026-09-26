package isometry

import (
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cameraSystem)(nil)

// cameraSystem carries out Turn and Follow on the cameras of this view, and every tick keeps each
// fastened camera behind its entity: centred on it at its altitude and turning, eased, until the
// way it walks runs up the screen. Only Follow given again, or the entity gone, lets it go.
type cameraSystem struct {
	turns     *control.Queue[Turn]
	tilts     *control.Queue[Tilt]
	follows   *control.Queue[Follow]
	drives    *control.Queue[Drive]
	selected  plugin.Tag[selection.Family]
	selecting bool // the selection was given: Follow has a unit to fasten to

	query    *goke.Query
	base     goke.Comp[world.Base]
	z        goke.OptComp[world.Z]
	marks    goke.OptComp[plugin.Tags[selection.Family]]
	driven   goke.OptComp[world.Driven]
	drivenID goke.CompID

	following []following
	released  []uid.UID64 // let go last tick, stopped by now: their Driven comes off
}

// following is one camera fastened behind one entity, and how its entity is driven this tick.
type following struct {
	cam    *isoCamera
	target uid.UID64
	drive  world.Driven
}

// shoulder is how far below the middle of the screen, as a part of its height, a fastened camera
// holds its entity looking along the ground; less the steeper it looks down, none straight down.
const shoulder = 0.25

// followEase is how long a fastened camera takes to turn most of the way behind its entity: about
// two thirds of any turn in that time.
const followEase = 250 * time.Millisecond

func (s *cameraSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base).Optional(&s.z).Optional(&s.marks).Optional(&s.driven).Build()
	s.drivenID = si.RegComp[world.Driven]()
}

func (s *cameraSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	for _, id := range s.released {
		cb.RemoveCompOne(id, s.drivenID)
	}
	s.released = s.released[:0]
	s.turns.Drain(func(i control.Issued[Turn]) {
		if cam, ok := i.Command.Camera.(*isoCamera); ok {
			cam.Turn(i.Command.Angle)
		}
	})
	s.tilts.Drain(func(i control.Issued[Tilt]) {
		if cam, ok := i.Command.Camera.(*isoCamera); ok {
			cam.Tilt(i.Command.Angle)
		}
	})
	s.follows.Drain(func(i control.Issued[Follow]) {
		cam, ok := i.Command.Camera.(*isoCamera)
		if !ok || s.letGo(cam) {
			return
		}
		if id, ok := s.theSelected(); ok {
			cb.AddOne(id, s.drivenID, world.Driven{})
			s.following = append(s.following, following{cam: cam, target: id})
			s.keep(&s.following[len(s.following)-1], 0)
		}
	})
	for k := range s.following {
		s.following[k].drive = world.Driven{}
	}
	s.drives.Drain(func(i control.Issued[Drive]) {
		for k := range s.following {
			if f := &s.following[k]; f.cam == i.Command.Camera {
				f.drive.Ahead = max(min(f.drive.Ahead+i.Command.Ahead, 1), -1)
				f.drive.Turn = max(min(f.drive.Turn+i.Command.Turn, 1), -1)
			}
		}
	})
	kept := s.following[:0]
	for _, f := range s.following {
		if s.keep(&f, d) {
			kept = append(kept, f)
		}
	}
	s.following = kept
}

// letGo unfastens cam and reports whether it was fastened; its entity is stopped now and no longer
// driven from the next tick.
func (s *cameraSystem) letGo(cam *isoCamera) bool {
	for i, f := range s.following {
		if f.cam == cam {
			s.write(f.target, world.Driven{Ahead: -1})
			s.released = append(s.released, f.target)
			s.following = append(s.following[:i], s.following[i+1:]...)
			return true
		}
	}
	return false
}

// write sets how id is driven, when it carries a Driven yet.
func (s *cameraSystem) write(id uid.UID64, in world.Driven) {
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		drivens := s.driven.Slice(cur)
		if drivens == nil {
			continue
		}
		for i, got := range cur.IDs {
			if got == id {
				drivens[i] = in
				return
			}
		}
	}
}

// theSelected is the one Selected entity; false with none, or several.
func (s *cameraSystem) theSelected() (uid.UID64, bool) {
	if !s.selecting {
		return 0, false
	}
	var one uid.UID64
	n := 0
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		marks := s.marks.Slice(cur)
		if marks == nil {
			continue
		}
		for i, m := range marks {
			if m.Has(s.selected) {
				one, n = cur.IDs[i], n+1
			}
		}
	}
	return one, n == 1
}

// keep turns f's camera, over d, towards the way its entity walks and centres it on the entity at
// its altitude; false when the entity is gone.
func (s *cameraSystem) keep(f *following, d time.Duration) bool {
	s.query.All()
	for s.query.Next() {
		cur := s.query.Cursor()
		for i, id := range cur.IDs {
			if id != f.target {
				continue
			}
			if drivens := s.driven.Slice(cur); drivens != nil {
				drivens[i] = f.drive
			}
			base := s.base.Slice(cur)[i]
			box := base.Pos.AABB
			alt := 0.0
			if zs := s.z.Slice(cur); zs != nil {
				alt = zs[i].Altitude
			}
			if dir := base.Vel.Dir; dir.X != 0 || dir.Y != 0 {
				if by := ease(behind(float32(dir.X), float32(dir.Y))-f.cam.Heading(), d); math.Abs(float64(by)) > 1e-4 {
					f.cam.Turn(by)
				}
			}
			f.cam.CenterOn((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2, alt)
			// over its shoulder: the lower the eye, the lower the entity on the screen, the more of
			// what lies ahead above it
			_, h := f.cam.Viewport()
			f.cam.Pan(0, -h*shoulder*float32(math.Cos(float64(f.cam.Pitch()))))
			return true
		}
	}
	return false
}

// behind is the heading from which the way (dx, dy) runs up the screen: the eye behind it.
func behind(dx, dy float32) float32 {
	return float32(math.Atan2(float64(-dx), float64(-dy))) - math.Pi/4
}

// ease is how much of the turn by, the shorter way round, a fastened camera makes over d; all of
// it for d zero, the first centring.
func ease(by float32, d time.Duration) float32 {
	by = float32(math.Remainder(float64(by), 2*math.Pi))
	if d <= 0 {
		return by
	}
	return by * float32(1-math.Exp(-float64(d)/float64(followEase)))
}
