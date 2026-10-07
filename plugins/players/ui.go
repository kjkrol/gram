package players

import (
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// Through is pl as whoever takes the input over a picture of the world on a scene's screen
// (ui.Image(feed).Input): told every frame where the picture lies, so the mouse over it is pl's,
// in the picture's pixels.
func (p *Plugin) Through(pl *Player) ui.Input { return through{p, pl} }

type through struct {
	p  *Plugin
	pl *Player
}

// Over is where pl's picture lies on the screen.
func (t through) Over(area geom.AABB) { t.pl.area = area }

// LookAt moves pl's camera to have the entity in the middle of its picture (cameras.LookAt).
func (t through) LookAt(entity uid.UID64) {
	if err := t.p.Issue(t.pl, cameras.LookAt{Camera: t.pl.Camera, Entity: entity}); err != nil {
		log.Printf("players: %v", err)
	}
}

// Shows reports whether pl's picture shows ui elements pinned to the entity: it is pl's, or
// nobody's.
func (t through) Shows(entity uid.UID64) bool {
	q := t.p.owned
	if q == nil || !q.Seek(entity) {
		return true
	}
	owners := t.p.ownedBy.At(q.Cursor())
	return owners == nil || *owners == 0 || owners.Has(owner.Of(t.pl.ID))
}

// owning builds the query Shows reads: whose every entity in the world is.
type owning struct{ p *Plugin }

func (o owning) Init(si *goke.SysInit) {
	o.p.owned = si.NewQueryBuilder(&o.p.ownedBase).Optional(&o.p.ownedBy).Build()
}

func (owning) Update(*goke.CmdBuf, time.Duration) {}

// IssueAs is Issue for pl: how a scene's buttons and keys give their commands (ui.Scene.Issue).
func (p *Plugin) IssueAs(pl *Player) func(cmd any) error {
	return func(cmd any) error { return p.Issue(pl, cmd) }
}
