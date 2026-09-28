package control

import (
	"reflect"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
)

// Mods is the modifier keys a trigger asks for, and at most one other key held down with them (see
// Holding); a trigger fires only with exactly these held.
type Mods struct {
	Shift, Ctrl, Alt bool

	held uint16 // the held key plus one; zero holds none, since ebiten.KeyA is zero
}

// Holding is m with key held down besides: S held while right-clicking, say.
func (m Mods) Holding(key ebiten.Key) Mods {
	m.held = uint16(key) + 1
	return m
}

// Held is the key m asks to be held down besides its modifiers, if any.
func (m Mods) Held() (ebiten.Key, bool) {
	if m.held == 0 {
		return 0, false
	}
	return ebiten.Key(m.held - 1), true
}

// Trigger is what fires a Binding: a key, a button, a gesture. The concrete triggers are values,
// so two bindings on one trigger are told apart when bound.
type Trigger interface{ trigger() }

// KeyPress fires when Key goes down with Mods held.
type KeyPress struct {
	Key  ebiten.Key
	Mods Mods
}

// KeyHeld fires once a tick while Key is down, whatever else is held — steering a vehicle, walking
// a character — its command landing in the tick after; a key tapped between two ticks never fires.
type KeyHeld struct{ Key ebiten.Key }

// ButtonPress fires when Button goes down with Mods held; Context.Cursor is where.
type ButtonPress struct {
	Button ebiten.MouseButton
	Mods   Mods
}

// Drag fires when Button comes up with Mods held, after going down: Context.Start is where it went
// down, Context.Cursor where it came up. A click is a Drag of no length.
type Drag struct {
	Button ebiten.MouseButton
	Mods   Mods
}

// Wheel fires on any scroll; Context.Wheel is how much.
type Wheel struct{}

// ButtonHeld fires every pass Button is down and the cursor moved inside the window;
// Context.Delta is by how much and Context.Start where the button went down.
type ButtonHeld struct{ Button ebiten.MouseButton }

// CursorAtEdge fires every tick the cursor rests near a window edge; the carrier says how near.
type CursorAtEdge struct{}

// CursorMove fires every pass the cursor moves, Context.Delta by how much: looking round with the
// mouse. It reaches the player the cursor is over, and one whose camera rides in an entity wherever
// the cursor is — the carrier captures it then, so it moves without end.
type CursorMove struct{}

func (KeyPress) trigger()     {}
func (KeyHeld) trigger()      {}
func (ButtonPress) trigger()  {}
func (Drag) trigger()         {}
func (Wheel) trigger()        {}
func (ButtonHeld) trigger()   {}
func (CursorAtEdge) trigger() {}
func (CursorMove) trigger()   {}

// Context is what a binding builds its command from: the player, its camera and this tick's input
// in screen pixels; World and WorldBox go through the camera.
type Context struct {
	Player      PlayerID
	Camera      camera.Camera
	Cursor      geom.Vec // where the cursor is, or where a button went down or up
	Start       geom.Vec // where a Drag began
	Delta       geom.Vec // cursor movement this tick
	Wheel       float64
	Screen      geom.Vec // the window's size in pixels
	Mods        Mods
	FillsScreen bool
}

// World is the ground point under screen position s: a camera.Picker's own Pick — a camera
// drawing heights finds the ground under the cursor itself, so a click on a hill lands on the
// hill — else the camera's FromScreen.
func (c Context) World(s geom.Vec) geom.Vec {
	sx, sy := float32(s.X), float32(s.Y)
	if p, ok := c.Camera.(camera.Picker); ok {
		x, y, _ := p.Pick(sx, sy)
		return geom.NewVec(float64(x), float64(y))
	}
	x, y := c.Camera.FromScreen(sx, sy)
	return geom.NewVec(float64(x), float64(y))
}

// WorldBox is the world rectangle between screen points a and b, at least one unit a side and no
// wider than what was dragged even across a wrapping seam; through a camera.Picker it spans the
// ground points under the two corners.
func (c Context) WorldBox(a, b geom.Vec) geom.AABB {
	var x0, y0, x1, y1 float32
	if _, picks := c.Camera.(camera.Picker); !picks {
		x0, y0, x1, y1 = camera.FromScreenRect(c.Camera, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y))
	} else {
		pa, pb := c.World(a), c.World(b)
		x0, y0, x1, y1 = float32(pa.X), float32(pa.Y), float32(pb.X), float32(pb.Y)
	}
	minX, maxX := float64(min(x0, x1)), float64(max(x0, x1))
	minY, maxY := float64(min(y0, y1)), float64(max(y0, y1))
	return geom.NewAABBAt(geom.NewVec(minX, minY), max(maxX-minX, 1), max(maxY-minY, 1))
}

// ScreenRect is the screen rectangle between a and b, at least a pixel a side.
func ScreenRect(a, b geom.Vec) geom.AABB {
	minX, maxX := min(a.X, b.X), max(a.X, b.X)
	minY, maxY := min(a.Y, b.Y), max(a.Y, b.Y)
	return geom.NewAABBAt(geom.NewVec(minX, minY), max(maxX-minX, 1), max(maxY-minY, 1))
}

// Binding is one thing a player can do: a Trigger, the command it issues and a label saying what
// it does, for a help screen; In has it hold in some of the camera's modes only. Build one with
// Command.
type Binding struct {
	Trigger Trigger
	Label   string

	command reflect.Type
	build   func(Context) (any, bool)
	modes   camera.Mode // zero: every mode
}

// In is b holding only while the player's camera is in one of modes (camera.ModeOf): one key may
// do one thing in the free camera and another riding in an entity. A binding never given modes
// holds in every one.
func (b Binding) In(modes ...camera.Mode) Binding {
	b.modes = 0
	for _, m := range modes {
		b.modes |= m
	}
	return b
}

// Holds reports whether b holds while the camera is in mode.
func (b Binding) Holds(mode camera.Mode) bool { return b.modes == 0 || b.modes&mode != 0 }

// Overlaps reports whether b and o hold in some mode both: two such bindings on one Trigger would
// both fire.
func (b Binding) Overlaps(o Binding) bool {
	return b.modes == 0 || o.modes == 0 || b.modes&o.modes != 0
}

// Command is a Binding issuing a C built from the Context when trigger fires; build may decline.
func Command[C any](trigger Trigger, label string, build func(c Context) (C, bool)) Binding {
	return Binding{Trigger: trigger, Label: label, command: reflect.TypeFor[C](), build: func(c Context) (any, bool) {
		cmd, ok := build(c)
		return cmd, ok
	}}
}

// Command is the type of command the binding issues; nil for one not built with Command.
func (b Binding) Command() reflect.Type { return b.command }

// Build is the command for c, if the binding has one to give.
func (b Binding) Build(c Context) (any, bool) {
	if b.build == nil {
		return nil, false
	}
	return b.build(c)
}
