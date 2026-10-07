package players

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/ui"
)

// Through is pl as whoever takes the input over a picture of the world on a scene's screen
// (ui.Image(feed).Input): told every frame where the picture lies, so the mouse over it is pl's,
// in the picture's pixels.
func (p *Plugin) Through(pl *Player) ui.Input { return through{pl} }

type through struct{ pl *Player }

// Over is where pl's picture lies on the screen.
func (t through) Over(area geom.AABB) { t.pl.area = area }

// IssueAs is Issue for pl: how a scene's buttons and keys give their commands (ui.Scene.Issue).
func (p *Plugin) IssueAs(pl *Player) func(cmd any) error {
	return func(cmd any) error { return p.Issue(pl, cmd) }
}
