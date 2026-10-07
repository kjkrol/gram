package players

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// capture catches the window's cursor while a local player's camera rides in an entity
// (camera.Inside) — the mouse looks round then, without end — and lets it go otherwise; it
// reports whether the cursor stayed as it was this pass, its move worth taking.
func (p *Plugin) capture() bool {
	riding := false
	for _, pl := range p.Locals() {
		riding = riding || camera.HowOf(pl.Camera) == camera.Inside
	}
	if riding == p.captured {
		return true
	}
	p.captured = riding
	set := p.setCapture
	if set == nil {
		set = control.CaptureCursor
	}
	set(riding)
	return false
}
