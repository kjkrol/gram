package players

import (
	"slices"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// false the pass the cursor is caught or let go: it jumps then; a menu of the players' up lets
// it go for the hand
func (p *Plugin) capture() bool {
	riding := false
	for _, pl := range p.read.Locals() {
		riding = riding || camera.HowOf(pl.pic.camera) == camera.Inside
	}
	riding = riding && !p.menuUp()
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

// menuUp reports whether the settings screen or the shortcuts are up
func (p *Plugin) menuUp() bool {
	return p.settings.shown ||
		p.composition != nil && slices.Contains(p.composition.Order(), ShortcutsName)
}
