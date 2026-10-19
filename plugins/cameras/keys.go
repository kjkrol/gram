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
// (KeyUnknown leaves one unbound), ZoomIn and ZoomOut pressed zoom about the cursor a ZoomStep,
// Wheel zooms about the cursor, Drag pans with the middle button, Edge scrolls with the cursor
// at a window edge, slantwise in a corner. Scrolling holds while the camera is outside any entity
// (camera.Outside).
type Keys struct {
	Up, Down, Left, Right control.Key
	ZoomIn, ZoomOut       control.Key
	ScrollSpeed           float32 // screen pixels a tick; zero is DefaultScrollSpeed
	Wheel, Drag, Edge     bool
	// CornersTurn has the cursor in a corner turn the camera the way Q and E do, a CornerStep a
	// tick, the left corners anticlockwise, the right ones clockwise, where Edge would scroll.
	CornersTurn bool
	// Farther and Nearer pressed with Ctrl have the camera see a SightStep farther and nearer.
	Farther, Nearer control.Key
	// Finer and Coarser pressed with Alt have the picture drawn at a ResolutionStep more and fewer
	// of its pixels.
	Finer, Coarser control.Key
}

// DefaultKeys are W, S, A and D, = and - to zoom, Ctrl+= and Ctrl+- to see farther and nearer,
// Alt+= and Alt+- for a finer and a coarser picture, the wheel, the middle drag and the edge.
func DefaultKeys() Keys {
	return Keys{
		Up: control.KeyW, Down: control.KeyS, Left: control.KeyA, Right: control.KeyD,
		ZoomIn: control.KeyEqual, ZoomOut: control.KeyMinus,
		Farther: control.KeyEqual, Nearer: control.KeyMinus,
		Finer: control.KeyEqual, Coarser: control.KeyMinus,
		Wheel: true, Drag: true, Edge: true,
	}
}

// Bindings are the keys bound to Pan and Zoom of the camera the player looks through.
func (k Keys) Bindings() []control.Binding {
	speed := k.ScrollSpeed
	if speed == 0 {
		speed = DefaultScrollSpeed
	}
	// the speed in the camera's pixels: the same on the screen at any resolution
	step := func(c control.Context) float32 { return float32(c.Pixels(float64(speed))) }
	scroll := func(dx, dy float32) func(control.Context) (Pan, bool) {
		return func(c control.Context) (Pan, bool) {
			s := step(c)
			return Pan{Camera: c.Camera, Dx: dx * s, Dy: dy * s}, true
		}
	}
	var out []control.Binding
	held := func(key control.Key, label string, dx, dy float32) {
		if key != control.KeyUnknown {
			out = append(out, control.Build(control.KeyHeld{Key: key}, label, scroll(dx, dy)).In(camera.Outside))
		}
	}
	held(k.Up, "Scroll up", 0, -1)
	held(k.Down, "Scroll down", 0, 1)
	held(k.Left, "Scroll left", -1, 0)
	held(k.Right, "Scroll right", 1, 0)
	zoom := func(key control.Key, label string, factor float32) {
		if key != control.KeyUnknown {
			out = append(out, control.Build(control.KeyPress{Key: key}, label,
				func(c control.Context) (Zoom, bool) {
					return Zoom{Camera: c.Camera, Factor: factor, At: c.World(c.Cursor)}, true
				}))
		}
	}
	zoom(k.ZoomIn, "Zoom in", ZoomStep)
	zoom(k.ZoomOut, "Zoom out", 1/ZoomStep)
	see := func(key control.Key, label string, factor float64) {
		if key != control.KeyUnknown {
			press := control.KeyPress{Key: key, Mods: control.Mods{Ctrl: true}}
			out = append(out, control.Build(press, label, func(c control.Context) (Sight, bool) {
				return Sight{Camera: c.Camera, Factor: factor}, true
			}))
		}
	}
	see(k.Farther, "See farther", SightStep)
	see(k.Nearer, "See nearer", 1/SightStep)
	draw := func(key control.Key, label string, factor float32) {
		if key != control.KeyUnknown {
			press := control.KeyPress{Key: key, Mods: control.Mods{Alt: true}}
			out = append(out, control.Build(press, label,
				func(c control.Context) (Resolution, bool) {
					return Resolution{Camera: c.Camera, Factor: factor}, true
				}))
		}
	}
	draw(k.Finer, "A finer picture", 1/ResolutionStep)
	draw(k.Coarser, "A coarser picture", ResolutionStep)
	if k.Wheel {
		out = append(out, control.Build(control.Wheel{}, "Zoom", func(c control.Context) (Zoom, bool) {
			if c.Wheel > 0 {
				return Zoom{Camera: c.Camera, Factor: ZoomStep, At: c.World(c.Cursor)}, true
			}
			return Zoom{Camera: c.Camera, Factor: 1 / ZoomStep, At: c.World(c.Cursor)}, true
		}))
	}
	if k.Drag {
		out = append(out, control.Build(control.ButtonHeld{Button: control.MouseButtonMiddle}, "Pan", func(c control.Context) (Pan, bool) {
			return Pan{Camera: c.Camera, Dx: float32(-c.Delta.X), Dy: float32(-c.Delta.Y)}, true
		}).In(camera.Outside))
	}
	if k.Edge {
		out = append(out, control.Build(control.CursorAtEdge{}, "Scroll", func(c control.Context) (Pan, bool) {
			var dx, dy float32
			side, speed := c.Edges(), step(c)
			if side.Corner != control.NoCorner {
				sx, sy := side.Corner.Signs()
				return Pan{Camera: c.Camera, Dx: sx * speed, Dy: sy * speed}, !k.CornersTurn
			}
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
		if k.CornersTurn {
			out = append(out, control.Build(control.CursorInCorner{},
				"Turn the world in a corner: anticlockwise on the left, clockwise on the right",
				func(c control.Context) (Turn, bool) {
					sx, _ := c.Edges().Corner.Signs()
					return Turn{Camera: c.Camera, Angle: sx * CornerStep}, sx != 0
				}).In(camera.Outside))
		}
	}
	return out
}
