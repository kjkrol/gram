package players

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

const (
	// EdgeMargin is how close to a window edge, in pixels, the cursor scrolls the camera.
	EdgeMargin = 30
	// EdgeDeadZone is the strip at the very edge that does not scroll, unless the window fills the
	// screen: a cursor parked there by the monitor's edge should not run away.
	EdgeDeadZone = 10
	// DefaultScrollSpeed is the edge scroll of CameraBindings, screen pixels a tick.
	DefaultScrollSpeed = 8
	// ZoomStep is the factor one wheel notch zooms by.
	ZoomStep = 1.1
)

// CameraBindings is the default camera control: wheel zooms about the cursor, a middle drag pans
// one to one with it, W, A, S and D held and the cursor at an edge scroll scrollSpeed pixels a
// tick (DefaultScrollSpeed) — up, left, down and right on the screen, however the view is turned.
// All but the wheel hold while the camera is outside any entity (camera.Outside), not riding in one.
func CameraBindings(scrollSpeed ...int32) []control.Binding {
	speed := float32(DefaultScrollSpeed)
	if len(scrollSpeed) > 0 {
		speed = float32(scrollSpeed[0])
	}
	scroll := func(dx, dy float32) func(control.Context) (Pan, bool) {
		return func(control.Context) (Pan, bool) { return Pan{Dx: dx * speed, Dy: dy * speed}, true }
	}
	return []control.Binding{
		control.Command(control.KeyHeld{Key: control.KeyW}, "Scroll up", scroll(0, -1)).In(camera.Outside),
		control.Command(control.KeyHeld{Key: control.KeyS}, "Scroll down", scroll(0, 1)).In(camera.Outside),
		control.Command(control.KeyHeld{Key: control.KeyA}, "Scroll left", scroll(-1, 0)).In(camera.Outside),
		control.Command(control.KeyHeld{Key: control.KeyD}, "Scroll right", scroll(1, 0)).In(camera.Outside),
		control.Command(control.Wheel{}, "Zoom", func(c control.Context) (Zoom, bool) {
			if c.Wheel > 0 {
				return Zoom{Factor: ZoomStep, At: c.World(c.Cursor)}, true
			}
			return Zoom{Factor: 1 / ZoomStep, At: c.World(c.Cursor)}, true
		}),
		control.Command(control.ButtonHeld{Button: control.MouseButtonMiddle}, "Pan", func(c control.Context) (Pan, bool) {
			return Pan{Dx: float32(-c.Delta.X), Dy: float32(-c.Delta.Y)}, true
		}).In(camera.Outside),
		control.Command(control.CursorAtEdge{}, "Scroll", func(c control.Context) (Pan, bool) {
			var dx, dy float32
			side := edgeSides(c)
			switch {
			case side.left:
				dx = -speed
			case side.right:
				dx = speed
			}
			switch {
			case side.top:
				dy = -speed
			case side.bottom:
				dy = speed
			}
			return Pan{Dx: dx, Dy: dy}, dx != 0 || dy != 0
		}).In(camera.Outside),
	}
}

type sides struct{ left, right, top, bottom bool }

// edgeSides is which window edges the cursor rests near, outside the dead zone.
func edgeSides(c control.Context) sides {
	dead := float64(EdgeDeadZone)
	if c.FillsScreen {
		dead = 0
	}
	x, y := c.Cursor.X, c.Cursor.Y
	return sides{
		left:   x >= dead && x < EdgeMargin,
		right:  x <= c.Screen.X-dead && x > c.Screen.X-EdgeMargin,
		top:    y >= dead && y < EdgeMargin,
		bottom: y <= c.Screen.Y-dead && y > c.Screen.Y-EdgeMargin,
	}
}

func atEdge(c control.Context) bool {
	s := edgeSides(c)
	return s.left || s.right || s.top || s.bottom
}
