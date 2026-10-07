package cameras

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cameraSystem)(nil)

// cameraSystem carries out Pan, Zoom, Follow and MouseLook on the cameras they name — a Pan lets
// go of whatever the camera was fastened to, a Zoom keeps it — and every tick keeps each camera
// fastened Centred over its entity, letting go of one that is gone. Cameras fastened Behind or
// Inside are the view plugin's.
type cameraSystem struct {
	p *Plugin

	query *goke.Query
	base  goke.Comp[world.Base]
	z     goke.OptComp[world.Z]
}

func (s *cameraSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base).Optional(&s.z).Build()
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
	s.p.looks.Drain(func(i control.Issued[MouseLook]) {
		if m, ok := i.Command.Camera.(camera.MouseLooker); ok {
			m.SetMouseLook(!m.MouseLook())
		}
	})
	s.keep()
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
		cur := s.query.Cursor()
		box := s.base.At(cur).Pos.AABB
		var alt float64
		if z := s.z.At(cur); z != nil {
			alt = z.Altitude
		}
		c.CenterOn((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2, alt)
	}
}
