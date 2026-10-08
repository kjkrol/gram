package players

import (
	"fmt"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// Through is pl as whoever takes the input over a picture of the world on a scene's screen
// (ui.Image(feed).Input): the picture pl acts through while the scene is active — the mouse over
// it pl's, in the picture's pixels, pl's keys and commands going through its camera.
func (p *Plugin) Through(pl *Player) ui.Input { return &through{p: p, pl: pl} }

type through struct {
	p   *Plugin
	pl  *Player
	cam camera.Camera // the camera of the picture, as last told
}

// Over wires pl to the picture lying over area, drawn through the camera of shown (a render.Feed):
// what the active scene tells before each pass of input. A player acting through two pictures in
// one pass panics: one scene gives it one view.
func (t *through) Over(area geom.AABB, shown render.Surface) {
	t.cam = nil
	if f, ok := shown.(interface{ Camera() camera.Camera }); ok {
		t.cam = f.Camera()
	}
	pl := t.pl
	if pl.wiredBy != nil && pl.wiredBy != t && pl.wiredIn == t.p.pass {
		panic(fmt.Sprintf("players: %s acts through two pictures at once: a scene gives a player one view", pl.Name))
	}
	pl.wiredBy, pl.wiredIn = t, t.p.pass
	pl.pic = picture{area: area, camera: t.cam}
}

// LookAt moves the camera of pl's picture to have the entity in its middle (cameras.LookAt).
func (t *through) LookAt(entity uid.UID64) {
	if t.cam == nil {
		return
	}
	if err := t.p.Issue(t.pl, cameras.LookAt{Camera: t.cam, Entity: entity}); err != nil {
		log.Printf("players: %v", err)
	}
}

// Shows reports whether pl's picture shows ui elements pinned to the entity: it is pl's, or
// nobody's.
func (t *through) Shows(entity uid.UID64) bool {
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
