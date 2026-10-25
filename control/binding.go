package control

import (
	"fmt"
	"reflect"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
)

// Mods is the modifier keys a trigger asks for, and at most one other key held down with them (see
// Holding); a trigger fires only with exactly these held.
type Mods struct {
	Shift, Ctrl, Alt bool

	held Key // KeyUnknown: none
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

// KeyDown fires when Key goes down, whatever other key is held; with Shift, Ctrl or Alt held only
// as a binding InChords: a jump while sprinting with Shift.
type KeyDown struct{ Key Key }

// KeyHeld fires once a tick while Key is down, whatever other key is held, with Shift, Ctrl or Alt
// only as a binding InChords; its command lands in the tick after. A key tapped between two ticks
// never fires.
type KeyHeld struct{ Key Key }

// KeyReleased fires when Key comes up, Context.Held saying how long it was down: a throw as
// strong as the key was held. A key held for a chord with modifiers (Shift+X) does not fire it,
// nor one let go with a modifier held besides it, unless the binding is InChords.
type KeyReleased struct{ Key Key }

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

// CursorInCorner fires every tick the cursor rests in a corner's square, as CursorAtEdge does
// too; Context.Edges says which corner.
type CursorInCorner struct{}

// CursorOver fires once a tick while the cursor lies over the player's picture of the world,
// still or not, its command landing in the tick after.
type CursorOver struct{}

// CursorMove fires every pass the cursor moves, Context.Delta by how much. It reaches the player
// the cursor is over, and one whose camera rides in an entity wherever the cursor is.
type CursorMove struct{}

func (KeyPress) trigger()       {}
func (KeyDown) trigger()        {}
func (KeyHeld) trigger()        {}
func (KeyReleased) trigger()    {}
func (ButtonPress) trigger()    {}
func (Drag) trigger()           {}
func (Wheel) trigger()          {}
func (ButtonHeld) trigger()     {}
func (CursorAtEdge) trigger()   {}
func (CursorInCorner) trigger() {}
func (CursorMove) trigger()     {}
func (CursorOver) trigger()     {}

// Context is what a binding builds its command from: the player, its camera and this tick's input
// in the camera's pixels, Resolution of them a shown one; World and WorldBox go through the
// camera.
type Context struct {
	Player      command.Issuer
	Camera      camera.Camera
	Cursor      geom.Vec // where the cursor is, or where a button went down or up
	Start       geom.Vec // where a Drag began
	Delta       geom.Vec
	Wheel       float32
	Screen      geom.Vec // the picture's size in the camera's pixels
	Resolution  float32  // camera pixels a shown pixel; 0 as 1
	Mods        Mods
	FillsScreen bool
	Held        time.Duration // how long a key was held, for KeyReleased
}

// Pixels is shown pixels in the camera's: a speed in pixels of the screen as the camera has it.
func (c Context) Pixels(shown float32) float32 { return shown * c.resolution() }

// Shown is v, in the camera's pixels, in the shown ones: a move of the mouse as the hand made it.
func (c Context) Shown(v geom.Vec) geom.Vec {
	k := c.resolution()
	return geom.NewVec(v.X/k, v.Y/k)
}

func (c Context) resolution() float32 {
	if c.Resolution <= 0 {
		return 1
	}
	return c.Resolution
}

// World is the ground point under screen position s: a camera.Picker's own Pick, else the
// camera's FromScreen. Through no camera it is s itself.
func (c Context) World(s geom.Vec) geom.Vec {
	if c.Camera == nil {
		return s
	}
	if p, ok := c.Camera.(camera.Picker); ok {
		x, y, _, _ := p.Pick(s.X, s.Y)
		return geom.NewVec(x, y)
	}
	return geom.NewVec(c.Camera.FromScreen(s.X, s.Y))
}

// Point is what screen position s sees and its height, through a camera.Picker's own Pick. Where
// the camera picks nothing (no Picker, or the sky) picked is false and at is World.
func (c Context) Point(s geom.Vec) (at geom.Vec, z float32, picked bool) {
	if c.Camera != nil {
		if p, ok := c.Camera.(camera.Picker); ok {
			x, y, h, ok := p.Pick(s.X, s.Y)
			if ok {
				return geom.NewVec(x, y), h, true
			}
		}
	}
	return c.World(s), 0, false
}

// WorldBox is the world rectangle between screen points a and b, at least one unit a side.
// Through no camera it is the screen's rectangle itself.
func (c Context) WorldBox(a, b geom.Vec) geom.AABB {
	var x0, y0, x1, y1 float32
	if c.Camera == nil {
		x0, y0, x1, y1 = a.X, a.Y, b.X, b.Y
	} else if _, picks := c.Camera.(camera.Picker); !picks {
		x0, y0, x1, y1 = camera.FromScreenRect(c.Camera, a.X, a.Y, b.X, b.Y)
	} else {
		pa, pb := c.World(a), c.World(b)
		x0, y0, x1, y1 = pa.X, pa.Y, pb.X, pb.Y
	}
	minX, maxX := min(x0, x1), max(x0, x1)
	minY, maxY := min(y0, y1), max(y0, y1)
	return geom.NewAABBAt(geom.NewVec(minX, minY), max(maxX-minX, 1), max(maxY-minY, 1))
}

const (
	// EdgeMargin is how close to a window edge, in pixels, the cursor rests at it (CursorAtEdge).
	EdgeMargin = 60
	// EdgeDeadZone is the strip at the very edge that is no edge, unless the window fills the
	// screen: a cursor parked there by the monitor's edge should not run away.
	EdgeDeadZone = 10
	// CornerReach is the side of the square at each corner, in pixels, within which the cursor
	// rests in the corner; CornerSide caps it for a small picture.
	CornerReach = 120
)

// CornerSide is the side of a corner's square in a picture w by h pixels: CornerReach, or a
// quarter of the shorter side where that is less.
func CornerSide(w, h float32) float32 { return min(CornerReach, min(w, h)/4) }

// Corner is which corner of the window a cursor rests in, NoCorner for none.
type Corner uint8

const (
	NoCorner Corner = iota
	TopLeft
	TopRight
	BottomLeft
	BottomRight
)

// Signs are the way the corner lies from the middle, -1 or 1 each way; 0 for none.
func (c Corner) Signs() (x, y float32) {
	switch c {
	case TopLeft:
		return -1, -1
	case TopRight:
		return 1, -1
	case BottomLeft:
		return -1, 1
	case BottomRight:
		return 1, 1
	}
	return 0, 0
}

// Edges are the window edges a cursor rests near, the corner it rests in, and how Deep in the
// strip or the corner's square it stands: 0 at the inner bound, 1 at the window's edge (the dead
// zone's), in a corner the deeper of the two ways.
type Edges struct {
	Left, Right, Top, Bottom bool
	Corner                   Corner
	Depth                    float32
}

// Any reports whether the cursor rests near any edge or in a corner.
func (e Edges) Any() bool { return e.Left || e.Right || e.Top || e.Bottom || e.Corner != NoCorner }

// Edges is which window edges the cursor rests near, outside the dead zone, and in which corner's
// square of CornerReach a side, the margins in shown pixels.
func (c Context) Edges() Edges {
	k := c.resolution()
	dead := float32(EdgeDeadZone) * k
	if c.FillsScreen {
		dead = 0
	}
	margin := float32(EdgeMargin) * k
	x, y := c.Cursor.X, c.Cursor.Y
	e := Edges{
		Left:   x >= dead && x < margin,
		Right:  x <= c.Screen.X-dead && x > c.Screen.X-margin,
		Top:    y >= dead && y < margin,
		Bottom: y <= c.Screen.Y-dead && y > c.Screen.Y-margin,
	}
	side := CornerSide(c.Screen.X/k, c.Screen.Y/k) * k
	nearLeft := x >= dead && x < side
	nearRight := x <= c.Screen.X-dead && x > c.Screen.X-side
	nearTop := y >= dead && y < side
	nearBottom := y <= c.Screen.Y-dead && y > c.Screen.Y-side
	switch {
	case nearLeft && nearTop:
		e.Corner = TopLeft
	case nearRight && nearTop:
		e.Corner = TopRight
	case nearLeft && nearBottom:
		e.Corner = BottomLeft
	case nearRight && nearBottom:
		e.Corner = BottomRight
	}
	// how far in from the window's edges, by the nearer each way
	inX, inY := min(x, c.Screen.X-x), min(y, c.Screen.Y-y)
	if e.Corner != NoCorner {
		e.Depth = max(depth(inX, side, dead), depth(inY, side, dead))
		return e
	}
	if e.Left || e.Right {
		e.Depth = depth(inX, margin, dead)
	}
	if e.Top || e.Bottom {
		e.Depth = max(e.Depth, depth(inY, margin, dead))
	}
	return e
}

// depth is how deep a cursor in pixels from the window's edge stands in a zone reach wide with
// the dead zone at its edge, 0 to 1
func depth(in, reach, dead float32) float32 {
	return min(max((reach-in)/(reach-dead), 0), 1)
}

// ScreenRect is the screen rectangle between a and b, at least a pixel a side.
func ScreenRect(a, b geom.Vec) geom.AABB {
	minX, maxX := min(a.X, b.X), max(a.X, b.X)
	minY, maxY := min(a.Y, b.Y), max(a.Y, b.Y)
	return geom.NewAABBAt(geom.NewVec(minX, minY), max(maxX-minX, 1), max(maxY-minY, 1))
}

// Binding is one thing a player can do: a Trigger, the command it issues and a label for a help
// screen. Make one with Build or Give.
type Binding struct {
	Trigger Trigger
	Label   string

	command reflect.Type
	build   func(Context) (command.Command, bool)
	hows    camera.How // zero: every How
	tool    Tool       // none: holds whatever the player holds
	chords  bool       // a key held, down or let go fires with a modifier held too
}

// In is b holding only while the player's camera is fastened one of hows (camera.HowOf). A
// binding never given hows holds whatever the camera does.
func (b Binding) In(hows ...camera.How) Binding {
	b.hows = 0
	for _, h := range hows {
		b.hows |= h
	}
	return b
}

// Holds reports whether b holds while the camera is fastened how.
func (b Binding) Holds(how camera.How) bool { return b.hows == 0 || b.hows&how != 0 }

// InChords is b, on a key held, down or let go, firing with Shift, Ctrl or Alt held too: a sprint
// with Shift, a jump while sprinting. Without it such a binding leaves the chords to the shortcuts.
func (b Binding) InChords() Binding {
	b.chords = true
	return b
}

// Takes reports whether b fires with mods held: a key's or a button's press and a drag as their
// triggers ask; a key held, down or let go with no modifier but the key itself, unless InChords.
func (b Binding) Takes(mods Mods) bool {
	var key Key
	switch t := b.Trigger.(type) {
	case KeyHeld:
		key = t.Key
	case KeyDown:
		key = t.Key
	case KeyReleased:
		key = t.Key
	default:
		return true
	}
	return b.chords || !(mods.Shift && key != KeyShift || mods.Ctrl && key != KeyControl ||
		mods.Alt && key != KeyAlt)
}

// With is b holding only while the player holds tool, its trigger taken from the bindings with
// no tool.
func (b Binding) With(tool Tool) Binding {
	b.tool = tool
	return b
}

// Tool is the tool b holds with; none for a binding holding whatever the player holds.
func (b Binding) Tool() Tool { return b.tool }

// Overlaps reports whether b and o hold in some How both with one tool: two such bindings on one
// Trigger would both fire.
func (b Binding) Overlaps(o Binding) bool {
	return b.tool == o.tool && (b.hows == 0 || o.hows == 0 || b.hows&o.hows != 0)
}

// Build is a Binding issuing a C built from the Context when trigger fires; build may decline.
func Build[C command.Command](trigger Trigger, label string, build func(c Context) (C, bool)) Binding {
	return Binding{Trigger: trigger, Label: label, command: reflect.TypeFor[C](), build: func(c Context) (command.Command, bool) {
		cmd, ok := build(c)
		return cmd, ok
	}}
}

// Contextual is a command that reads where it was given: In is the command for the Context its
// trigger fired in — the one pointed at with the cursor.
type Contextual interface {
	In(c Context) command.Command
}

// Give is a Binding issuing cmd whenever trigger fires, a Contextual one for the Context. A
// command kept in a register (a ceremony.command.Command) must come from it: one made on the spot panics.
func Give(trigger Trigger, label string, cmd command.Command) Binding {
	if d, ok := cmd.(interface{ Defined() bool }); ok && !d.Defined() {
		panic(fmt.Sprintf("control: the key for %q is given a command no register holds: define it by name first (world.Commands) and give what Named hands back", label))
	}
	cmd = command.Unwrap(cmd)
	return Binding{Trigger: trigger, Label: label, command: reflect.TypeOf(cmd), build: func(c Context) (command.Command, bool) {
		if in, ok := cmd.(Contextual); ok {
			return in.In(c), true
		}
		return cmd, true
	}}
}

// Type is the type of command the binding issues; nil for one made with neither Build nor Give.
func (b Binding) Type() reflect.Type { return b.command }

// Build is the command for c, if the binding has one to give.
func (b Binding) Build(c Context) (command.Command, bool) {
	if b.build == nil {
		return nil, false
	}
	return b.build(c)
}
