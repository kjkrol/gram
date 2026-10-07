package players

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

var _ goke.System = (*cameraSystem)(nil)

// cameraSystem carries out the Pan and Zoom commands on each issuing player's camera: a Pan lets
// go of whatever the camera was fastened to, a Zoom keeps it.
type cameraSystem struct{ p *Plugin }

func (s *cameraSystem) Init(*goke.SysInit) {}

func (s *cameraSystem) Update(*goke.CmdBuf, time.Duration) {
	s.p.pans.Drain(func(i control.Issued[Pan]) {
		pl := s.p.ByID(i.Player)
		if pl == nil {
			return
		}
		if f, ok := pl.Camera.(camera.Fastenable); ok && f.Fastening().How != 0 {
			f.Fasten(camera.Fastening{})
		}
		pl.Camera.Pan(i.Command.Dx, i.Command.Dy)
	})
	s.p.zooms.Drain(func(i control.Issued[Zoom]) {
		pl := s.p.ByID(i.Player)
		if pl == nil {
			return
		}
		x, y := float32(i.Command.At.X), float32(i.Command.At.Y)
		if i.Command.Factor >= 1 {
			pl.Camera.ZoomIn(i.Command.Factor, x, y)
		} else {
			pl.Camera.ZoomOut(1/i.Command.Factor, x, y)
		}
	})
}
