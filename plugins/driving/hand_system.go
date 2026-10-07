package driving

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*handSystem)(nil)

// handSystem sums the tick's commands into hands and writes every unit's Driven from the hand on
// it — a player's whose camera the command came through is fastened to it, a player's that
// selected it with that camera fastened to nothing, or its own — marking it Driving; a Driving
// unit with no hand on it this tick is written a zero hand, to brake. The fields of a Driven an
// eye writes — Look, Flown, Climb — are left alone.
type handSystem struct {
	hands    *hands
	selected tag.Tag[selection.Family]
	*units

	drivenID goke.CompID
	statesID goke.CompID
}

func (s *handSystem) Init(si *goke.SysInit) {
	s.build(si)
	s.drivenID = si.RegComp[steering.Driven]()
	s.statesID = si.RegComp[tag.Tags[States]]()
}

func (s *handSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.hands.sum()
	for s.query.All(); s.query.Next(); {
		cur := s.query.Cursor()
		drivens, states, owners, marks := s.driven.Slice(cur), s.states.Slice(cur), s.owners.Slice(cur), s.marks.Slice(cur)
		for i, id := range cur.IDs {
			var owned tag.Tags[owner.Family]
			if owners != nil {
				owned = owners[i]
			}
			h, on := s.on(id, owned, marks != nil && marks[i].Has(s.selected))
			var in steering.Driven
			if drivens != nil {
				in = drivens[i]
			}
			switch {
			case on:
				in.Ahead, in.Turn, in.Sprint, in.Face = h.ahead, h.turn, h.sprint, h.way // driveSystem normalises Face
				if drivens != nil {
					drivens[i] = in
				} else {
					cb.AddOne(id, s.drivenID, in)
				}
				if states != nil {
					states[i] = states[i].With(Driving)
				} else {
					cb.AddOne(id, s.statesID, tag.Tags[States](0).With(Driving))
				}
			case drivens != nil && states != nil && states[i].Has(Driving):
				in.Ahead, in.Turn, in.Sprint, in.Face = 0, 0, false, geom.Vec{}
				drivens[i] = in
			}
		}
	}
}

// on is the hand on unit id — a player's, through a camera fastened to it or, the camera fastened
// to nothing, while the unit is selected, the unit obeying the player (owner.Obeys); else its own.
func (s *handSystem) on(id uid.UID64, owned tag.Tags[owner.Family], selected bool) (hand, bool) {
	for _, h := range s.hands.players {
		if !owner.Obeys(owned, h.player) {
			continue
		}
		f := camera.Fastening{}
		if c, ok := h.camera.(camera.Fastenable); ok {
			f = c.Fastening()
		}
		if f.How != 0 && f.Entity == id || f.How == 0 && selected {
			return h.hand, true
		}
	}
	h, ok := s.hands.entities[id]
	return h, ok
}
