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
		x, y := i.Command.At.X, i.Command.At.Y
		if i.Command.Factor >= 1 {
			cam.ZoomIn(i.Command.Factor, x, y)
		} else {
			cam.ZoomOut(1/i.Command.Factor, x, y)
		}
	})
	s.p.sights.Drain(func(i command.Issued[Sight]) {
		if cam, ok := i.Command.Camera.(seeing); ok {
			cam.See(i.Command.Factor)
		}
	})
	s.p.resolutions.Drain(func(i command.Issued[Resolution]) {
		cam := i.Command.Camera
		if i.Command.Factor > 0 && s.p.apply(cam, i.Command.Factor) {
			s.p.still(ProfileOf(cam))
			s.p.note(cam)
		}
	})
	s.p.adapts.Drain(func(i command.Issued[Adapt]) {
		s.p.write.SetStrategy(i.Command.Profile, i.Command.Strategy)
	})
	s.p.pictures.Drain(func(i command.Issued[Picture]) {
		s.p.write.SetResolution(i.Command.Profile, i.Command.Resolution)
	})
	s.p.sharps.Drain(func(i command.Issued[SharpFigures]) {
		s.p.write.SetSharpFigures(i.Command.Profile, i.Command.On)
	})
	s.p.layers.Drain(func(i command.Issued[Layers]) {
		s.p.write.SetLayers(i.Command.Profile, i.Command.On)
	})
	s.p.layerShares.Drain(func(i command.Issued[LayerShares]) {
		s.p.write.SetLayerShares(i.Command.Profile, i.Command.Inner, i.Command.Middle)
	})
	s.p.layerRes.Drain(func(i command.Issued[LayerResolutions]) {
		s.p.write.SetLayerResolutions(i.Command.Profile, i.Command.Outer, i.Command.Middle)
	})
	s.p.budgets.Drain(func(i command.Issued[Budget]) { s.p.write.SetBudget(i.Command.Frame) })
	s.p.samplings.Drain(func(i command.Issued[Sampling]) { s.p.write.SetSampling(i.Command.Over) })
	s.p.thresholds.Drain(func(i command.Issued[Thresholds]) {
		s.p.write.SetThresholds(i.Command.Under, i.Command.Met)
	})
	s.p.showLayers.Drain(func(i command.Issued[ShowLayers]) { s.p.write.ShowLayers(i.Command.Profile, i.Command.Shown) })
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
	s.p.centerAts.Drain(func(i command.Issued[CenterAt]) {
		cam := i.Command.Camera
		if cam == nil {
			return
		}
		if f, ok := cam.(camera.Fastenable); ok && f.Fastening().How != 0 {
			f.Fasten(camera.Fastening{})
		}
		var z float32
		if s.p.ground != nil {
			z = s.p.ground(i.Command.X, i.Command.Y)
		}
		cam.CenterOn(i.Command.X, i.Command.Y, z)
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
	var alt float32
	if z := s.z.At(cur); z != nil {
		alt = z.Altitude
	}
	cam.CenterOn((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2, alt)
}
