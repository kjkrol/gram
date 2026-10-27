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
// Wheel zooms about the cursor, Drag pans with the middle button, Grab with the right, Edge
// scrolls with the cursor at a window edge, slantwise in a corner. Scrolling, dragging, grabbing
// and the zones hold where the hand is free (camera.Free): behind a unit the keys drive it and
// the cursor at an edge lets go of no camera kept there.
type Keys struct {
	Up, Down, Left, Right control.Key
	ZoomIn, ZoomOut       control.Key
	ScrollSpeed           float32 // screen pixels a tick; zero is DefaultScrollSpeed
	Wheel, Drag, Edge     bool
	// CornersTurn has the cursor in a corner turn the camera the way Q and E do, a CornerStep a
	// tick, where Edge would scroll: the bottom-left and the top-right corners anticlockwise, the
	// bottom-right and the top-left clockwise, the world under the cursor sliding along the top
	// or the bottom edge towards the middle.
	CornersTurn bool
	// Grab has the right button grab the world and drag it past a click's slop, yielding to
	// whatever else takes the right drag: a selection's orders. A game's choice, no default.
	Grab bool
	// Farther and Nearer pressed with Ctrl have the camera see a SightStep farther and nearer.
	Farther, Nearer control.Key
}

// DefaultKeys are W, S, A and D, = and - to zoom, Ctrl+= and Ctrl+- to see farther and nearer,
// the wheel, the middle drag and the edge; no grab.
func DefaultKeys() Keys {
	return Keys{
		Up: control.KeyW, Down: control.KeyS, Left: control.KeyA, Right: control.KeyD,
		ZoomIn: control.KeyEqual, ZoomOut: control.KeyMinus,
		Farther: control.KeyEqual, Nearer: control.KeyMinus,
		Wheel: true, Drag: true, Edge: true,
	}
}

// Bindings are the keys bound to Pan and Zoom of the camera the player looks through.
func (k Keys) Bindings() []control.Binding { return k.bindings(nil) }

// bindings are the keys' bindings, the edge's as p's settings say where p is given: scrolling
// or still, at the speed set
func (k Keys) bindings(p *Plugin) []control.Binding {
	speed := k.ScrollSpeed
	if speed == 0 {
		speed = DefaultScrollSpeed
	}
	// the speed in the camera's pixels: the same on the screen at any resolution
	step := func(c control.Context) float32 { return c.Pixels(speed) }
	edgeStep := func(c control.Context) float32 {
		if p != nil && p.read.EdgeSpeed() > 0 {
			return c.Pixels(p.read.EdgeSpeed())
		}
		return step(c)
	}
	scroll := func(dx, dy float32) func(control.Context) (Pan, bool) {
		return func(c control.Context) (Pan, bool) {
			s := step(c)
			return Pan{Camera: c.Camera, Dx: dx * s, Dy: dy * s}, true
		}
	}
	var out []control.Binding
	held := func(key control.Key, label string, dx, dy float32) {
		if key != control.KeyUnknown {
			out = append(out, control.Build(control.KeyHeld{Key: key}, label, scroll(dx, dy)).
				In(camera.Free))
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
	see := func(key control.Key, label string, factor float32) {
		if key != control.KeyUnknown {
			press := control.KeyPress{Key: key, Mods: control.Mods{Ctrl: true}}
			out = append(out, control.Build(press, label, func(c control.Context) (Sight, bool) {
				return Sight{Camera: c.Camera, Factor: factor}, true
			}))
		}
	}
	see(k.Farther, "See farther", SightStep)
	see(k.Nearer, "See nearer", 1/SightStep)
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
			return Pan{Camera: c.Camera, Dx: -c.Delta.X, Dy: -c.Delta.Y}, true
		}).In(camera.Free))
	}
	if k.Grab {
		out = append(out, control.Build(control.ButtonHeld{Button: control.MouseButtonRight},
			"Grab the world and drag it, with nothing selected",
			func(c control.Context) (Pan, bool) {
				if !c.Dragged() {
					return Pan{}, false
				}
				// the first pass past a click's slop takes the whole way: the world catches up
				before := c
				before.Cursor = c.Cursor.Sub(c.Delta)
				d := c.Delta
				if !before.Dragged() {
					d = c.Cursor.Sub(c.Start)
				}
				return Pan{Camera: c.Camera, Dx: -d.X, Dy: -d.Y}, true
			}).In(camera.Free).Yielding())
	}
	if k.Edge {
		out = append(out, control.Build(control.CursorAtEdge{}, "Scroll", func(c control.Context) (Pan, bool) {
			if p != nil && !p.read.EdgesScroll() {
				return Pan{}, false
			}
			// the deeper in the zone, the faster: a quarter the speed at its inner bound
			var dx, dy float32
			side := c.Edges()
			speed := edgeStep(c) * (0.25 + 0.75*side.Depth)
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
		}).In(camera.Free))
		if k.CornersTurn {
			out = append(out, control.Build(control.CursorInCorner{},
				"Turn the world in a corner: anticlockwise in the bottom-left and the top-right, "+
					"clockwise in the other two",
				func(c control.Context) (Turn, bool) {
					if p != nil && !p.read.EdgesScroll() {
						return Turn{}, false
					}
					// the corners of a diagonal turn alike: the top ones the other way round from
					// the bottom ones, so the world under the cursor slides towards the middle
					sx, sy := c.Edges().Corner.Signs()
					return Turn{Camera: c.Camera, Angle: sx * sy * CornerStep}, sx != 0
				}).In(camera.Free))
		}
	}
	return out
}
