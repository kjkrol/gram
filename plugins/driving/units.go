package driving

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

// units is the one query both systems of the driving walk — every entity steered by a profile,
// with whatever of the rest it carries — built by whichever is set up first.
type units struct {
	query  *goke.Query
	base   goke.Comp[world.Base]
	steer  goke.Comp[steering.Steering]
	course goke.OptComp[steering.Course]
	driven goke.OptComp[steering.Driven]
	at     goke.OptComp[unit.At]
	mover  goke.OptComp[unit.Mover]
	owners goke.OptComp[tag.Tags[owner.Family]]
	marks  goke.OptComp[tag.Tags[selection.Family]]
	states goke.OptComp[tag.Tags[States]]
	places goke.OptComp[tag.Tags[unit.States]]
}

func (u *units) build(si *goke.SysInit) {
	if u.query != nil {
		return
	}
	u.query = si.NewQueryBuilder(&u.base, &u.steer).
		Optional(&u.course).Optional(&u.driven).Optional(&u.at).Optional(&u.mover).
		Optional(&u.owners).Optional(&u.marks).Optional(&u.states).Optional(&u.places).
		Build()
}
