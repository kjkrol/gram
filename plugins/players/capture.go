package players

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
)

// capture catches the window's cursor while a local player's camera rides in an entity
// (camera.FirstPerson) — the mouse looks round then, without end — and lets it go otherwise; it
// reports whether the cursor stayed as it was this pass, its move worth taking.
func (p *Plugin) capture() bool {
	riding := false
	for _, pl := range p.Locals() {
		riding = riding || camera.ModeOf(pl.Camera) == camera.FirstPerson
	}
	if riding == p.captured {
		return true
	}
	p.captured = riding
	set := p.setCapture
	if set == nil {
		set = captureCursor
	}
	set(riding)
	return false
}

// captureCursor catches the window's cursor, or shows it again.
func captureCursor(on bool) {
	if on {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		return
	}
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
}
