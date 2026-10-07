package players

import (
	"log"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/cameras"
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

// IssueAs is Issue for pl: how a scene's buttons and keys give their commands (ui.Scene.Issue).
func (p *Plugin) IssueAs(pl *Player) func(cmd any) error {
	return func(cmd any) error { return p.Issue(pl, cmd) }
}
