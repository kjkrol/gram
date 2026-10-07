package cameras

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

const (
	// DefaultScrollSpeed is how fast the keys and the edge scroll, screen pixels a tick.
	DefaultScrollSpeed = 8
	// ZoomStep is the factor one wheel notch zooms by.
	ZoomStep = 1.1
)

// Keys are a player's keys to its camera: Up, Down, Left and Right held scroll it on the screen
// (KeyUnknown leaves one unbound), Wheel zooms about the cursor, Drag pans with the middle button,
// Edge scrolls with the cursor at a window edge. Scrolling holds while the camera is outside any
// entity (camera.Outside).
type Keys struct {
	Up, Down, Left, Right control.Key
	ScrollSpeed           float32 // screen pixels a tick; zero is DefaultScrollSpeed
	Wheel, Drag, Edge     bool
}

// DefaultKeys are W, S, A and D, the wheel, the middle drag and the edge.
func DefaultKeys() Keys {
	return Keys{Up: control.KeyW, Down: control.KeyS, Left: control.KeyA, Right: control.KeyD, Wheel: true, Drag: true, Edge: true}
}

// Bindings are the keys bound to Pan and Zoom of the camera the player looks through.
func (k Keys) Bindings() []control.Binding {
	speed := k.ScrollSpeed
	if speed == 0 {
		speed = DefaultScrollSpeed
	}
	scroll := func(dx, dy float32) func(control.Context) (Pan, bool) {
		return func(c control.Context) (Pan, bool) {
			return Pan{Camera: c.Camera, Dx: dx * speed, Dy: dy * speed}, true
		}
	}
	var out []control.Binding
	held := func(key control.Key, label string, dx, dy float32) {
		if key != control.KeyUnknown {
			out = append(out, control.Command(control.KeyHeld{Key: key}, label, scroll(dx, dy)).In(camera.Outside))
		}
	}
	held(k.Up, "Scroll up", 0, -1)
	held(k.Down, "Scroll down", 0, 1)
	held(k.Left, "Scroll left", -1, 0)
	held(k.Right, "Scroll right", 1, 0)
	if k.Wheel {
		out = append(out, control.Command(control.Wheel{}, "Zoom", func(c control.Context) (Zoom, bool) {
			if c.Wheel > 0 {
				return Zoom{Camera: c.Camera, Factor: ZoomStep, At: c.World(c.Cursor)}, true
			}
			return Zoom{Camera: c.Camera, Factor: 1 / ZoomStep, At: c.World(c.Cursor)}, true
		}))
	}
	if k.Drag {
		out = append(out, control.Command(control.ButtonHeld{Button: control.MouseButtonMiddle}, "Pan", func(c control.Context) (Pan, bool) {
			return Pan{Camera: c.Camera, Dx: float32(-c.Delta.X), Dy: float32(-c.Delta.Y)}, true
		}).In(camera.Outside))
	}
	if k.Edge {
		out = append(out, control.Command(control.CursorAtEdge{}, "Scroll", func(c control.Context) (Pan, bool) {
			var dx, dy float32
			side := c.Edges()
			switch {
			case side.Left:
				dx = -speed
			case side.Right:
				dx = speed
			}
			switch {
			case side.Top:
				dy = -speed
			case side.Bottom:
				dy = speed
			}
			return Pan{Camera: c.Camera, Dx: dx, Dy: dy}, dx != 0 || dy != 0
		}).In(camera.Outside))
	}
	return out
}
