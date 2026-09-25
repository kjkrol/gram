package players

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
)

// translator turns one tick's input into commands: every local player's bindings, in order.
type translator struct{ p *Plugin }

var _ control.EventHandler = translator{}

func (t translator) HandleEvents(ev *control.InputEvents) {
	mods := control.Mods{Shift: ev.Modifiers.Shift, Ctrl: ev.Modifiers.Ctrl, Alt: ev.Modifiers.Alt}
	if t.p.ground == nil {
		if g := t.p.worldPlugin.Ground(); g != nil {
			t.p.ground = func(x, y float32) float32 { return float32(g.At(geom.NewVec(float64(x), float64(y)))) }
		}
	}
	for _, pl := range t.p.Locals() {
		pl.cursor = ev.MousePos
		ctx := control.Context{Player: pl.ID, Camera: pl.Camera, Cursor: ev.MousePos, Delta: ev.CursorDelta,
			Wheel: ev.ScrollDelta, Screen: pl.screen(), Mods: mods, FillsScreen: ev.WindowFillsScreen, Ground: t.p.ground}

		for _, k := range ev.KeyEvents {
			switch k.Action {
			case control.ActionPress:
				t.fire(pl, control.KeyPress{Key: k.Key, Mods: pl.withHeld(mods)}, ctx)
				pl.keyDown(k.Key)
			case control.ActionRelease:
				pl.keyUp(k.Key)
			}
		}
		mods := pl.withHeld(mods)
		ctx.Mods = mods
		for _, c := range ev.ClickQueue {
			pl.cursor = c.Pos
			at := ctx
			at.Cursor = c.Pos
			switch c.Action {
			case control.ActionPress:
				pl.press(c.Button, c.Pos)
				at.Start = c.Pos
				t.fire(pl, control.ButtonPress{Button: c.Button, Mods: mods}, at)
			case control.ActionRelease:
				if start, down := pl.release(c.Button); down {
					at.Start = start
					t.fire(pl, control.Drag{Button: c.Button, Mods: mods}, at)
				}
			}
		}
		if ev.ScrollDelta != 0 {
			t.fire(pl, control.Wheel{}, ctx)
		}

		inside := ev.MousePos.X >= 0 && ev.MousePos.X < ctx.Screen.X && ev.MousePos.Y >= 0 && ev.MousePos.Y < ctx.Screen.Y
		if !inside {
			continue
		}
		if ev.CursorDelta.X != 0 || ev.CursorDelta.Y != 0 {
			for button := range pl.held {
				t.fire(pl, control.ButtonHeld{Button: button}, ctx)
			}
			if _, down := pl.held[ebiten.MouseButtonMiddle]; ev.MiddleDown && !down {
				t.fire(pl, control.ButtonHeld{Button: ebiten.MouseButtonMiddle}, ctx)
			}
		}
		if atEdge(ctx) {
			t.fire(pl, control.CursorAtEdge{}, ctx)
		}
	}
}

// fire issues the command of every binding of pl on trigger that builds one.
func (t translator) fire(pl *Player, trigger control.Trigger, ctx control.Context) {
	for _, b := range pl.bindings {
		if b.Trigger != trigger {
			continue
		}
		if cmd, ok := b.Build(ctx); ok {
			if err := t.p.Issue(pl, cmd); err != nil {
				panic(err)
			}
		}
	}
}
