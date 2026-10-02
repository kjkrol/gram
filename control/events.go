package control

import (
	"github.com/kjkrol/aabbworld/geom"
)

type KeyAction int

const (
	ActionPress KeyAction = iota
	ActionRelease
)

type KeyEvent struct {
	Key    Key
	Action KeyAction
}

type ClickEvent struct {
	Pos    geom.Vec
	Button MouseButton
	Action KeyAction
}

type InputEvents struct {
	MousePos    geom.Vec
	CursorDelta geom.Vec
	Modifiers   struct {
		Shift, Ctrl, Alt bool
	}
	MiddleDown bool
	// WindowFillsScreen reports whether the window covers the whole monitor, fullscreen or borderless.
	WindowFillsScreen bool

	ClickQueue  []ClickEvent
	KeyEvents   []KeyEvent
	ScrollDelta float64
}

func (e *InputEvents) ResetTransient() {
	e.ClickQueue = e.ClickQueue[:0]
	e.KeyEvents = e.KeyEvents[:0]
	e.ScrollDelta = 0
	e.CursorDelta = geom.Vec{}
}

func (e *InputEvents) AddKeyEvent(key Key, action KeyAction) {
	e.KeyEvents = append(e.KeyEvents, KeyEvent{Key: key, Action: action})
}

// AddClickEvent takes plain int for caller ergonomics — stored as geom.Vec.
func (e *InputEvents) AddClickEvent(x, y int, button MouseButton, action KeyAction) {
	e.ClickQueue = append(e.ClickQueue, ClickEvent{
		Pos:    geom.NewVec(float64(x), float64(y)),
		Button: button,
		Action: action,
	})
}

// cursorCapture is the window's way to catch the cursor, the engine's; nil without a window.
var cursorCapture func(on bool)

// SetCursorCapture is the engine's: how the window catches its cursor and lets it go.
func SetCursorCapture(fn func(on bool)) { cursorCapture = fn }

// CaptureCursor catches the window's cursor — hidden, the mouse moving without end — or shows it
// again; nothing without a window.
func CaptureCursor(on bool) {
	if cursorCapture != nil {
		cursorCapture(on)
	}
}
