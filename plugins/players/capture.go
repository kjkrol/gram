package players

import (
	"slices"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// false the pass the cursor is caught or let go: it jumps then; a menu of the players' up lets
// it go for the hand. The mouse looks round inside a unit and turns the camera behind one
func (p *Plugin) capture() bool {
	riding := false
	for _, pl := range p.read.Locals() {
		riding = riding || looksRound(pl.pic.camera)
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

// looksRound reports whether the mouse moves cam's view: riding inside a unit or behind it
func looksRound(cam camera.Camera) bool {
	return camera.HowOf(cam)&(camera.Inside|camera.Behind) != 0
}

// menuUp reports whether the settings screen or the controls screen is up
func (p *Plugin) menuUp() bool {
	return p.settings.shown ||
		p.composition != nil && slices.Contains(p.composition.Order(), ControlsName)
}
