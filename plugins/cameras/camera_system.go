package cameras

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cameraSystem)(nil)

// cameraSystem carries out Pan, Zoom, Follow, LookAt and MouseLook on the cameras they name — a Pan lets
// go of whatever the camera was fastened to, a Zoom keeps it — and every tick keeps each camera
// fastened Centred over its entity, letting go of one that is gone. Cameras fastened Behind or
// Inside are the view plugin's.
type cameraSystem struct {
	p *Plugin

	query    *goke.Query
	base     goke.Comp[world.Base]
	z        goke.OptComp[world.Z]
	labelled *goke.Query // every entity called something: whom a camera starts fastened to
	label    goke.Comp[entity.Label]
}

func (s *cameraSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base).Optional(&s.z).Build()
	s.labelled = si.NewQueryBuilder(&s.label).Build()
}

func (s *cameraSystem) Update(*goke.CmdBuf, time.Duration) {
	s.p.pans.Drain(func(i control.Issued[Pan]) {
		cam := i.Command.Camera
		if cam == nil {
			return
		}
		if f, ok := cam.(camera.Fastenable); ok && f.Fastening().How != 0 {
			f.Fasten(camera.Fastening{})
		}
		cam.Pan(i.Command.Dx, i.Command.Dy)
	})
	s.p.zooms.Drain(func(i control.Issued[Zoom]) {
		cam := i.Command.Camera
		if cam == nil {
			return
		}
		x, y := float32(i.Command.At.X), float32(i.Command.At.Y)
		if i.Command.Factor >= 1 {
			cam.ZoomIn(i.Command.Factor, x, y)
		} else {
			cam.ZoomOut(1/i.Command.Factor, x, y)
		}
	})
	s.p.follows.Drain(func(i control.Issued[Follow]) {
		cam, ok := i.Command.Camera.(camera.Fastenable)
		if !ok {
			return
		}
		if i.ByEntity {
			s.fasten(cam, i.Entity)
			return
		}
		if !i.Command.On || cam.Fastening().How != 0 {
			cam.Fasten(camera.Fastening{})
			return
		}
		s.fasten(cam, i.Command.Entity)
	})
	s.p.lookAts.Drain(func(i control.Issued[LookAt]) {
		if cam := i.Command.Camera; cam != nil && s.query.Seek(i.Command.Entity) {
			if f, ok := cam.(camera.Fastenable); ok && f.Fastening().How != 0 {
				f.Fasten(camera.Fastening{})
			}
			s.centre(cam)
		}
	})
	s.p.looks.Drain(func(i control.Issued[MouseLook]) {
		if m, ok := i.Command.Camera.(camera.MouseLooker); ok {
			m.SetMouseLook(!m.MouseLook())
		}
	})
	s.start()
	s.keep()
}

// start fastens Centred every camera waiting for the entity its config names, once it is in the
// world.
func (s *cameraSystem) start() {
	if len(s.p.starts) == 0 {
		return
	}
	waiting := s.p.starts[:0]
	for _, st := range s.p.starts {
		if id, ok := s.called(st.whom); ok {
			st.cam.Fasten(camera.Fastening{Entity: id, How: camera.Centred})
		} else {
			waiting = append(waiting, st)
		}
	}
	s.p.starts = waiting
}

// called is the entity whom names, when one is in the world.
func (s *cameraSystem) called(whom entity.Whom) (uid.UID64, bool) {
	for s.labelled.All(); s.labelled.Next(); {
		cur := s.labelled.Cursor()
		for i, l := range s.label.Slice(cur) {
			if whom.Holds(l) {
				return cur.IDs[i], true
			}
		}
	}
	return 0, false
}

// fasten fastens cam Centred over id, when id is in the world.
func (s *cameraSystem) fasten(cam camera.Fastenable, id uid.UID64) {
	if s.query.Seek(id) {
		cam.Fasten(camera.Fastening{Entity: id, How: camera.Centred})
	}
}

// keep centres every camera fastened Centred on its entity, at its altitude; a camera fastened
// to an entity that is gone is let go.
func (s *cameraSystem) keep() {
	for _, c := range s.p.cameras {
		cam, ok := c.(camera.Fastenable)
		if !ok {
			continue
		}
		f := cam.Fastening()
		if f.How != camera.Centred {
			continue
		}
		if !s.query.Seek(f.Entity) {
			cam.Fasten(camera.Fastening{})
			continue
		}
		s.centre(c)
	}
}

// centre centres cam on the entity under the query's cursor, at its altitude.
func (s *cameraSystem) centre(cam camera.Camera) {
	cur := s.query.Cursor()
	box := s.base.At(cur).Pos.AABB
	var alt float64
	if z := s.z.At(cur); z != nil {
		alt = z.Altitude
	}
	cam.CenterOn((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2, alt)
}
