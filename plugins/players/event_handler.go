package players

import (
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

type eventHandler struct{ p *Plugin }

var _ control.EventHandler = eventHandler{}

func (t eventHandler) HandleEvents(ev *control.InputEvents) {
	t.p.tools()
	mods := control.Mods{Shift: ev.Modifiers.Shift, Ctrl: ev.Modifiers.Ctrl, Alt: ev.Modifiers.Alt}
	// the pass the cursor is caught or let go it jumps: no move is taken from it
	settled := t.p.capture()
	for _, pl := range t.p.Locals() {
		under := covers(pl, ev.MousePos)
		if under {
			pl.in.cursor = localPoint(pl, ev.MousePos)
		}
		ctx := control.Context{Player: pl.ID, Camera: pl.pic.camera, Cursor: pl.in.cursor, Delta: ev.CursorDelta,
			Wheel: ev.ScrollDelta, Screen: screenOf(pl), Mods: mods, FillsScreen: ev.WindowFillsScreen}

		for _, k := range ev.KeyEvents {
			switch k.Action {
			case control.ActionPress:
				t.fire(pl, control.KeyPress{Key: k.Key, Mods: pl.in.withHeld(mods)}, ctx)
				t.fire(pl, control.KeyDown{Key: k.Key}, ctx)
				pl.in.keyDown(pl.bindings, k.Key)
				pl.in.steer(pl.bindings, k.Key, true)
			case control.ActionRelease:
				pl.in.keyUp(k.Key)
				pl.in.steer(pl.bindings, k.Key, false)
			}
		}
		pl.in.last = ctx
		mods := pl.in.withHeld(mods)
		ctx.Mods = mods
		moved := ev.CursorDelta.X != 0 || ev.CursorDelta.Y != 0
		if moved && settled && (under || camera.HowOf(pl.pic.camera) == camera.Inside) {
			t.fire(pl, control.CursorMove{}, ctx)
		}
		for _, c := range ev.ClickQueue {
			at := ctx
			switch c.Action {
			case control.ActionPress:
				if !covers(pl, c.Pos) {
					continue
				}
				pos := localPoint(pl, c.Pos)
				pl.in.cursor, at.Cursor, at.Start = pos, pos, pos
				pl.in.press(c.Button, pos)
				t.fire(pl, control.ButtonPress{Button: c.Button, Mods: mods}, at)
			case control.ActionRelease:
				if start, down := pl.in.release(c.Button); down {
					pos := localPoint(pl, c.Pos)
					pl.in.cursor, at.Cursor, at.Start = pos, pos, start
					t.fire(pl, control.Drag{Button: c.Button, Mods: mods}, at)
				}
			}
		}
		pl.in.over = false
		if !under {
			continue
		}
		if ev.ScrollDelta != 0 {
			t.fire(pl, control.Wheel{}, ctx)
		}

		inside := ctx.Cursor.X >= 0 && ctx.Cursor.X < ctx.Screen.X && ctx.Cursor.Y >= 0 && ctx.Cursor.Y < ctx.Screen.Y
		pl.in.over = inside
		if !inside {
			continue
		}
		if ev.CursorDelta.X != 0 || ev.CursorDelta.Y != 0 {
			for button, start := range pl.in.held {
				at := ctx
				at.Start = start
				t.fire(pl, control.ButtonHeld{Button: button}, at)
			}
			if _, down := pl.in.held[control.MouseButtonMiddle]; ev.MiddleDown && !down {
				t.fire(pl, control.ButtonHeld{Button: control.MouseButtonMiddle}, ctx)
			}
		}
		if ctx.Edges().Any() {
			t.fire(pl, control.CursorAtEdge{}, ctx)
		}
	}
}

func (t eventHandler) fire(pl *Player, trigger control.Trigger, ctx control.Context) {
	how := camera.HowOf(pl.pic.camera)
	for _, b := range pl.bindings {
		if b.Trigger != trigger || !pl.fires(b, how) {
			continue
		}
		if cmd, ok := b.Build(ctx); ok {
			if err := t.p.Issue(pl, cmd); err != nil {
				panic(err)
			}
		}
	}
}

func (t eventHandler) hold() {
	for _, pl := range t.p.Locals() {
		for _, key := range pl.in.steering {
			t.fire(pl, control.KeyHeld{Key: key}, pl.in.last)
		}
		if pl.in.over {
			t.fire(pl, control.CursorOver{}, pl.in.last)
		}
	}
}

type input struct {
	cursor   geom.Vec // in the player's picture's pixels
	held     map[control.MouseButton]geom.Vec
	keys     []control.Key
	steering []control.Key
	last     control.Context
	over     bool
}

func (in *input) press(button control.MouseButton, at geom.Vec) {
	if in.held == nil {
		in.held = map[control.MouseButton]geom.Vec{}
	}
	in.held[button] = at
}

func (in *input) release(button control.MouseButton) (geom.Vec, bool) {
	at, down := in.held[button]
	delete(in.held, button)
	return at, down
}

func (in *input) keyDown(bindings []control.Binding, key control.Key) {
	if !holds(bindings, key) {
		return
	}
	in.keyUp(key)
	in.keys = append(in.keys, key)
}

func (in *input) keyUp(key control.Key) {
	if i := slices.Index(in.keys, key); i >= 0 {
		in.keys = slices.Delete(in.keys, i, i+1)
	}
}

func (in *input) steer(bindings []control.Binding, key control.Key, down bool) {
	if i := slices.Index(in.steering, key); i >= 0 {
		in.steering = slices.Delete(in.steering, i, i+1)
	}
	if !down {
		return
	}
	for _, b := range bindings {
		if t, ok := b.Trigger.(control.KeyHeld); ok && t.Key == key {
			in.steering = append(in.steering, key)
			return
		}
	}
}

func (in *input) withHeld(mods control.Mods) control.Mods {
	if len(in.keys) == 0 {
		return mods
	}
	return mods.Holding(in.keys[len(in.keys)-1])
}

func holds(bindings []control.Binding, key control.Key) bool {
	for _, b := range bindings {
		var mods control.Mods
		switch t := b.Trigger.(type) {
		case control.KeyPress:
			mods = t.Mods
		case control.ButtonPress:
			mods = t.Mods
		case control.Drag:
			mods = t.Mods
		default:
			continue
		}
		if k, ok := mods.Held(); ok && k == key {
			return true
		}
	}
	return false
}

func screenOf(pl *Player) geom.Vec {
	if cam := pl.pic.camera; cam != nil {
		w, h := cam.Viewport()
		return geom.NewVec(float64(w), float64(h))
	}
	return pl.pic.area.BottomRight.Sub(pl.pic.area.TopLeft)
}

func covers(pl *Player, at geom.Vec) bool {
	if pl.pic.camera == nil {
		return false
	}
	a := pl.pic.area
	return a == (geom.AABB{}) || at.X >= a.TopLeft.X && at.X < a.BottomRight.X && at.Y >= a.TopLeft.Y && at.Y < a.BottomRight.Y
}

func localPoint(pl *Player, at geom.Vec) geom.Vec {
	return geom.NewVec(at.X-pl.pic.area.TopLeft.X, at.Y-pl.pic.area.TopLeft.Y)
}
