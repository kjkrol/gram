package control

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
)

// Mods is the modifier keys a trigger asks for, and at most one other key held down with them (see
// Holding); a trigger fires only with exactly these held.
type Mods struct {
	Shift, Ctrl, Alt bool

	held Key // the key held besides; KeyUnknown holds none
}

// Holding is m with key held down besides: S held while right-clicking, say.
func (m Mods) Holding(key Key) Mods {
	m.held = key
	return m
}

// Held is the key m asks to be held down besides its modifiers, if any.
func (m Mods) Held() (Key, bool) {
	return m.held, m.held != KeyUnknown
}

// Trigger is what fires a Binding: a key, a button, a gesture. The concrete triggers are values,
// so two bindings on one trigger are told apart when bound.
type Trigger interface{ trigger() }

// KeyPress fires when Key goes down with Mods held.
type KeyPress struct {
	Key  Key
	Mods Mods
}

// KeyHeld fires once a tick while Key is down, whatever else is held — steering a vehicle, walking
// a character — its command landing in the tick after; a key tapped between two ticks never fires.
type KeyHeld struct{ Key Key }

// ButtonPress fires when Button goes down with Mods held; Context.Cursor is where.
type ButtonPress struct {
	Button MouseButton
	Mods   Mods
}

// Drag fires when Button comes up with Mods held, after going down: Context.Start is where it went
// down, Context.Cursor where it came up. A click is a Drag of no length.
type Drag struct {
	Button MouseButton
	Mods   Mods
}

// Wheel fires on any scroll; Context.Wheel is how much.
type Wheel struct{}

// ButtonHeld fires every pass Button is down and the cursor moved inside the window;
// Context.Delta is by how much and Context.Start where the button went down.
type ButtonHeld struct{ Button MouseButton }

// CursorAtEdge fires every tick the cursor rests near a window edge; the carrier says how near.
type CursorAtEdge struct{}

// CursorOver fires once a tick while the cursor lies over the player's picture of the world, still
// or not, its command landing in the tick after: what the cursor points at as the camera moves
// under it (selection's Hover).
type CursorOver struct{}

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
func (CursorOver) trigger()   {}

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
// hill — else the camera's FromScreen; through no camera, s itself.
func (c Context) World(s geom.Vec) geom.Vec {
	if c.Camera == nil {
		return s
	}
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
// ground points under the two corners; through no camera, the screen's rectangle itself.
func (c Context) WorldBox(a, b geom.Vec) geom.AABB {
	var x0, y0, x1, y1 float32
	if c.Camera == nil {
		x0, y0, x1, y1 = float32(a.X), float32(a.Y), float32(b.X), float32(b.Y)
	} else if _, picks := c.Camera.(camera.Picker); !picks {
		x0, y0, x1, y1 = camera.FromScreenRect(c.Camera, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y))
	} else {
		pa, pb := c.World(a), c.World(b)
		x0, y0, x1, y1 = float32(pa.X), float32(pa.Y), float32(pb.X), float32(pb.Y)
	}
	minX, maxX := float64(min(x0, x1)), float64(max(x0, x1))
	minY, maxY := float64(min(y0, y1)), float64(max(y0, y1))
	return geom.NewAABBAt(geom.NewVec(minX, minY), max(maxX-minX, 1), max(maxY-minY, 1))
}

const (
	// EdgeMargin is how close to a window edge, in pixels, the cursor rests at it (CursorAtEdge).
	EdgeMargin = 30
	// EdgeDeadZone is the strip at the very edge that is no edge, unless the window fills the
	// screen: a cursor parked there by the monitor's edge should not run away.
	EdgeDeadZone = 10
)

// Edges are the window edges a cursor rests near.
type Edges struct{ Left, Right, Top, Bottom bool }

// Any reports whether the cursor rests near any edge.
func (e Edges) Any() bool { return e.Left || e.Right || e.Top || e.Bottom }

// Edges is which window edges the cursor rests near, outside the dead zone.
func (c Context) Edges() Edges {
	dead := float64(EdgeDeadZone)
	if c.FillsScreen {
		dead = 0
	}
	x, y := c.Cursor.X, c.Cursor.Y
	return Edges{
		Left:   x >= dead && x < EdgeMargin,
		Right:  x <= c.Screen.X-dead && x > c.Screen.X-EdgeMargin,
		Top:    y >= dead && y < EdgeMargin,
		Bottom: y <= c.Screen.Y-dead && y > c.Screen.Y-EdgeMargin,
	}
}

// ScreenRect is the screen rectangle between a and b, at least a pixel a side.
func ScreenRect(a, b geom.Vec) geom.AABB {
	minX, maxX := min(a.X, b.X), max(a.X, b.X)
	minY, maxY := min(a.Y, b.Y), max(a.Y, b.Y)
	return geom.NewAABBAt(geom.NewVec(minX, minY), max(maxX-minX, 1), max(maxY-minY, 1))
}

// Binding is one thing a player can do: a Trigger, the command it issues and a label saying what
// it does, for a help screen; In has it hold in some of the camera's fastenings only. Build one with
// Command.
type Binding struct {
	Trigger Trigger
	Label   string

	command reflect.Type
	build   func(Context) (any, bool)
	hows    camera.How // zero: every How
}

// In is b holding only while the player's camera is fastened one of hows (camera.HowOf): one key
// may do one thing with the camera loose and another riding in an entity. A binding never given
// hows holds whatever the camera does.
func (b Binding) In(hows ...camera.How) Binding {
	b.hows = 0
	for _, h := range hows {
		b.hows |= h
	}
	return b
}

// Holds reports whether b holds while the camera is fastened how.
func (b Binding) Holds(how camera.How) bool { return b.hows == 0 || b.hows&how != 0 }

// Overlaps reports whether b and o hold in some How both: two such bindings on one Trigger would
// both fire.
func (b Binding) Overlaps(o Binding) bool {
	return b.hows == 0 || o.hows == 0 || b.hows&o.hows != 0
}

// Command is a Binding issuing a C built from the Context when trigger fires; build may decline.
func Command[C any](trigger Trigger, label string, build func(c Context) (C, bool)) Binding {
	return Binding{Trigger: trigger, Label: label, command: reflect.TypeFor[C](), build: func(c Context) (any, bool) {
		cmd, ok := build(c)
		return cmd, ok
	}}
}

// Contextual is a command that reads where it was given: In is the command for the Context its
// trigger fired in — the one pointed at with the cursor.
type Contextual interface{ In(c Context) any }

// Give is a Binding issuing cmd whenever trigger fires: a command the game named beforehand. One
// that is Routed is given as its handler takes it, one that is Contextual for the Context. A
// command kept in a register (a rule.Command, the world's Commands) must come from it: one made
// on the spot panics.
func Give(trigger Trigger, label string, cmd any) Binding {
	if d, ok := cmd.(interface{ Defined() bool }); ok && !d.Defined() {
		panic(fmt.Sprintf("control: the key for %q is given a command no register holds: define it by name first (world.Commands) and give what Named hands back", label))
	}
	cmd = Unwrap(cmd)
	return Binding{Trigger: trigger, Label: label, command: reflect.TypeOf(cmd), build: func(c Context) (any, bool) {
		if in, ok := cmd.(Contextual); ok {
			return in.In(c), true
		}
		return cmd, true
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
