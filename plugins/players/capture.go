package players

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// false the pass the cursor is caught or let go: it jumps then
func (p *Plugin) capture() bool {
	riding := false
	for _, pl := range p.Locals() {
		riding = riding || camera.HowOf(pl.pic.camera) == camera.Inside
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
