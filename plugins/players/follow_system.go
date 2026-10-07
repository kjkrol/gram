package players

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*followSystem)(nil)

// followSystem carries out the Follows — a player's fastens its camera Centred over the one unit
// it has chosen (Chooser), or lets go; an entity's fastens its owner's camera over it — and every
// tick keeps each camera fastened Centred over its entity, letting go of one that is gone.
type followSystem struct {
	p *Plugin

	query  *goke.Query
	base   goke.Comp[world.Base]
	z      goke.OptComp[world.Z]
	owners goke.OptComp[tag.Tags[owner.Family]]
}

func (s *followSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base).Optional(&s.z).Optional(&s.owners).Build()
}

func (s *followSystem) Update(*goke.CmdBuf, time.Duration) {
	s.p.follows.Drain(func(i control.Issued[Follow]) {
		if i.ByEntity {
			s.followOwn(i.Entity)
		} else if pl := s.p.ByID(i.Player); pl != nil {
			s.toggle(pl)
		}
	})
	s.keep()
}

// toggle lets pl's camera go when it is fastened, else fastens it over the one unit pl has
// chosen; with none chosen, or no Chooser, nothing.
func (s *followSystem) toggle(pl *Player) {
	cam, ok := pl.Camera.(camera.Fastenable)
	if !ok {
		return
	}
	if cam.Fastening().How != 0 {
		cam.Fasten(camera.Fastening{})
		return
	}
	if s.p.chooser == nil {
		return
	}
	if id, ok := s.p.chooser.Chosen(pl.ID); ok {
		cam.Fasten(camera.Fastening{Entity: id, How: camera.Centred})
	}
}

// followOwn fastens the camera of id's owner over id; an entity nobody owns, or whose owner has
// no camera to fasten, is left alone.
func (s *followSystem) followOwn(id uid.UID64) {
	if !s.query.Seek(id) {
		return
	}
	owned := s.owners.At(s.query.Cursor())
	if owned == nil {
		return
	}
	for _, pl := range s.p.players {
		if !owned.Has(owner.Of(pl.ID)) {
			continue
		}
		if cam, ok := pl.Camera.(camera.Fastenable); ok {
			cam.Fasten(camera.Fastening{Entity: id, How: camera.Centred})
		}
		return
	}
}

// keep centres every camera fastened Centred on its entity, at its altitude; a camera fastened
// to an entity that is gone is let go. Cameras fastened Behind or Inside are the view plugin's.
func (s *followSystem) keep() {
	for _, pl := range s.p.players {
		cam, ok := pl.Camera.(camera.Fastenable)
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
		pl.Camera.CenterOn((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2, alt)
	}
}
