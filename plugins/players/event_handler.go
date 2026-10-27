package players

import (
	"slices"
	"time"

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
	for _, pl := range t.p.read.Locals() {
		under := ev.CursorInWindow && covers(pl, ev.MousePos) && !pl.pic.covered
		if under {
			pl.in.cursor = localPoint(pl, ev.MousePos)
		}
		k := camera.ResolutionOf(pl.pic.camera)
		ctx := control.Context{
			Player:     pl.ID,
			Camera:     pl.pic.camera,
			Cursor:     scaled(pl.in.cursor, k),
			Delta:      scaled(ev.CursorDelta, k),
			Wheel:      ev.ScrollDelta,
			Screen:     screenOf(pl),
			Resolution: k,
			Mods:       mods,
		}

		for _, k := range ev.KeyEvents {
			switch k.Action {
			case control.ActionPress:
				chord := t.fire(pl, control.KeyPress{Key: k.Key, Mods: pl.in.withHeld(mods)}, ctx) &&
					(mods.Shift || mods.Ctrl || mods.Alt)
				t.fire(pl, control.KeyDown{Key: k.Key}, ctx)
				pl.in.keyDown(pl.bindings, k.Key)
				pl.in.steer(pl.bindings, k.Key, true)
				pl.in.pressed(pl.bindings, k.Key, t.p.now(), chord)
			case control.ActionRelease:
				pl.in.keyUp(k.Key)
				pl.in.steer(pl.bindings, k.Key, false)
				if held, ok := pl.in.released(k.Key, t.p.now()); ok {
					up := ctx
					up.Held = held
					t.fire(pl, control.KeyReleased{Key: k.Key}, up)
				}
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
				pl.in.cursor, at.Cursor, at.Start = pos, scaled(pos, k), scaled(pos, k)
				pl.in.press(c.Button, pos)
				t.fire(pl, control.ButtonPress{Button: c.Button, Mods: mods}, at)
			case control.ActionRelease:
				if start, down := pl.in.release(c.Button); down {
					pos := localPoint(pl, c.Pos)
					pl.in.cursor, at.Cursor, at.Start = pos, scaled(pos, k), scaled(start, k)
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
				at.Start = scaled(start, k)
				t.fire(pl, control.ButtonHeld{Button: button}, at)
			}
			if _, down := pl.in.held[control.MouseButtonMiddle]; ev.MiddleDown && !down {
				t.fire(pl, control.ButtonHeld{Button: control.MouseButtonMiddle}, ctx)
			}
		}
		if edges := ctx.Edges(); edges.Any() {
			t.fire(pl, control.CursorAtEdge{}, ctx)
			if edges.Corner != control.NoCorner {
				t.fire(pl, control.CursorInCorner{}, ctx)
			}
		}
	}
}

// fire issues what the bindings on trigger build, the yielding ones only when no other did; true
// when any did
func (t eventHandler) fire(pl *Player, trigger control.Trigger, ctx control.Context) bool {
	how := camera.HowOf(pl.pic.camera)
	for _, yielding := range []bool{false, true} {
		fired := false
		for _, b := range pl.bindings {
			if b.Trigger != trigger || b.Yields() != yielding || !pl.fires(b, how) ||
				!b.Takes(ctx.Mods) {
				continue
			}
			if cmd, ok := b.Build(ctx); ok {
				if err := t.p.write.Issue(pl, cmd); err != nil {
					panic(err)
				}
				fired = true
			}
		}
		if fired {
			return true
		}
	}
	return false
}

func (t eventHandler) hold() {
	for _, pl := range t.p.read.Locals() {
		for _, key := range pl.in.steering {
			t.fire(pl, control.KeyHeld{Key: key}, pl.in.last)
		}
		if pl.in.over {
			t.fire(pl, control.CursorOver{}, pl.in.last)
		}
	}
}

type input struct {
	cursor   geom.Vec // in the player's picture's shown pixels
	held     map[control.MouseButton]geom.Vec
	keys     []control.Key
	steering []control.Key
	down     map[control.Key]keyDown // keys a KeyReleased watches, since when
	last     control.Context
	over     bool
}

type keyDown struct {
	at    time.Time
	chord bool // a chord with modifiers fired while it was down: no release of its own
}

// a key a KeyReleased watches goes down; a chord fired marks every other one held
func (in *input) pressed(bindings []control.Binding, key control.Key, now time.Time, chord bool) {
	if chord {
		for k, d := range in.down {
			if k != key {
				d.chord = true
				in.down[k] = d
			}
		}
	}
	for _, b := range bindings {
		if b.Trigger == control.Trigger(control.KeyReleased{Key: key}) {
			if in.down == nil {
				in.down = map[control.Key]keyDown{}
			}
			if _, already := in.down[key]; !already { // a key repeating as it is held
				in.down[key] = keyDown{at: now}
			}
			return
		}
	}
}

// how long key was down, false where nothing watches it or a chord took it
func (in *input) released(key control.Key, now time.Time) (time.Duration, bool) {
	d, ok := in.down[key]
	delete(in.down, key)
	return now.Sub(d.at), ok && !d.chord
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
		return geom.NewVec(w, h)
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

// shown pixels in the camera's, k of them a shown one
func scaled(v geom.Vec, k float32) geom.Vec {
	return geom.NewVec(v.X*k, v.Y*k)
}
