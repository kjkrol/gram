package cameras

import (
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cameraSystem)(nil)

type cameraSystem struct {
	p *Plugin

	query    *goke.Query
	base     goke.Comp[world.Base]
	z        goke.OptComp[world.Z]
	labelled *goke.Query
	label    goke.Comp[ceremony.Label]
}

func (s *cameraSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base).Optional(&s.z).Build()
	s.labelled = si.NewQueryBuilder(&s.label).Build()
}

func (s *cameraSystem) Update(*goke.CmdBuf, time.Duration) {
	s.p.pans.Drain(func(i command.Issued[Pan]) {
		cam := i.Command.Camera
		if cam == nil {
			return
		}
		if f, ok := cam.(camera.Fastenable); ok && f.Fastening().How != 0 {
			f.Fasten(camera.Fastening{})
		}
		cam.Pan(i.Command.Dx, i.Command.Dy)
	})
	s.p.zooms.Drain(func(i command.Issued[Zoom]) {
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
	s.p.follows.Drain(func(i command.Issued[Follow]) {
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
	s.p.centerOns.Drain(func(i command.Issued[CenterOn]) {
		if cam := i.Command.Camera; cam != nil && s.query.Seek(i.Command.Entity) {
			if f, ok := cam.(camera.Fastenable); ok && f.Fastening().How != 0 {
				f.Fasten(camera.Fastening{})
			}
			s.centre(cam)
		}
	})
	s.start()
	s.keep()
}

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

func (s *cameraSystem) called(whom ceremony.Whom) (uid.UID64, bool) {
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

func (s *cameraSystem) fasten(cam camera.Fastenable, id uid.UID64) {
	if s.query.Seek(id) {
		cam.Fasten(camera.Fastening{Entity: id, How: camera.Centred})
	}
}

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

func (s *cameraSystem) centre(cam camera.Camera) {
	cur := s.query.Cursor()
	box := s.base.At(cur).Pos.AABB
	var alt float64
	if z := s.z.At(cur); z != nil {
		alt = z.Altitude
	}
	cam.CenterOn((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2, alt)
}
