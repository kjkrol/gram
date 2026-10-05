package players

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/players/owner"
)

var _ goke.System = (*giveSystem)(nil)

// giveSystem carries out the Gives: the one owner of the entity that gave it.
type giveSystem struct {
	gives    *control.Queue[Give]
	query    *goke.Query
	owners   goke.Comp[tag.Tags[owner.Family]]
	ownersID goke.CompID
}

func (s *giveSystem) Init(si *goke.SysInit) {
	s.ownersID = si.RegComp[tag.Tags[owner.Family]]()
	s.query = si.NewQueryBuilder(&s.owners).Build()
}

func (s *giveSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.gives.Drain(func(i control.Issued[Give]) {
		if !i.ByEntity || int(i.Command.To) > owner.Players {
			return
		}
		var owners tag.Tags[owner.Family]
		if i.Command.To != control.Nobody {
			owners = owners.With(owner.Of(i.Command.To))
		}
		if s.query.Seek(i.Entity) {
			*s.owners.At(s.query.Cursor()) = owners
		} else if owners != 0 {
			cb.AddOne(i.Entity, s.ownersID, owners)
		}
	})
}
