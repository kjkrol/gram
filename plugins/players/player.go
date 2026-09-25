package players

import (
	"fmt"
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world"
)

// Player is whoever acts in the game and may look at a part of it: a camera and the View through
// it, and — at this keyboard — the bindings that turn its input into commands.
type Player struct {
	ID     control.PlayerID
	Name   string
	Camera camera.Camera
	View   *world.View

	world    *world.Plugin
	own      bool            // looks through a camera of its own, saved with the game
	area     image.Rectangle // its part of the screen, as last laid out; empty is all of it
	local    bool
	bindings []control.Binding
	cursor   geom.Vec
	held     map[ebiten.MouseButton]geom.Vec // buttons down and where they went down
	keys     []ebiten.Key                    // keys down that some binding holds, last pressed last
	steering []ebiten.Key                    // keys down that some KeyHeld binding is on
}

// OwnCamera gives the player a camera of its own over the world, and its View, saved with the
// game; call before Use. Two local players with cameras of their own split the screen.
func (p *Player) OwnCamera() *Player {
	p.Camera = p.world.NewCamera()
	p.View = p.world.ViewFor(p.Camera)
	p.own = true
	return p
}

// Area is the player's part of the screen, as the viewports last laid it out; empty before.
func (p *Player) Area() image.Rectangle { return p.area }

// Bind adds bindings to the player; two on one Trigger are an error, never a silent last-one-wins.
func (p *Player) Bind(bindings ...control.Binding) error {
	for _, b := range bindings {
		if b.Command() == nil {
			return fmt.Errorf("players: %q is not a Binding built with control.Command", b.Label)
		}
		for _, have := range p.bindings {
			if have.Trigger == b.Trigger {
				return fmt.Errorf("players: %q and %q are both bound to %v for %s", have.Label, b.Label, b.Trigger, p.Name)
			}
		}
		p.bindings = append(p.bindings, b)
	}
	return nil
}

// Bindings lists what the player can do, in the order bound.
func (p *Player) Bindings() []control.Binding { return p.bindings }

// DragBox is the drag in progress of a button the player has a Drag binding on, in screen pixels.
func (p *Player) DragBox() (start, current geom.Vec, dragging bool) {
	for _, b := range p.bindings {
		d, ok := b.Trigger.(control.Drag)
		if !ok {
			continue
		}
		if at, down := p.held[d.Button]; down {
			return at, p.cursor, true
		}
	}
	return geom.Vec{}, geom.Vec{}, false
}

// screen is the size of the player's part of the screen, in pixels, as the camera shows the world.
func (p *Player) screen() geom.Vec {
	w, h := p.Camera.Viewport()
	return geom.NewVec(float64(w), float64(h))
}

// covers reports whether the screen point at is in the player's part of the screen.
func (p *Player) covers(at geom.Vec) bool {
	return p.area.Empty() || image.Pt(int(at.X), int(at.Y)).In(p.area)
}

// localPoint is the screen point at in the pixels of the player's part of the screen.
func (p *Player) localPoint(at geom.Vec) geom.Vec {
	return geom.NewVec(at.X-float64(p.area.Min.X), at.Y-float64(p.area.Min.Y))
}

func (p *Player) press(button ebiten.MouseButton, at geom.Vec) {
	if p.held == nil {
		p.held = map[ebiten.MouseButton]geom.Vec{}
	}
	p.held[button] = at
}

// keyDown notes key as held when some binding of the player asks for it held.
func (p *Player) keyDown(key ebiten.Key) {
	if !p.holds(key) {
		return
	}
	p.keyUp(key)
	p.keys = append(p.keys, key)
}

// steer notes key as down when a KeyHeld binding of the player is on it, or forgets it.
func (p *Player) steer(key ebiten.Key, down bool) {
	if i := slices.Index(p.steering, key); i >= 0 {
		p.steering = slices.Delete(p.steering, i, i+1)
	}
	if !down {
		return
	}
	for _, b := range p.bindings {
		if t, ok := b.Trigger.(control.KeyHeld); ok && t.Key == key {
			p.steering = append(p.steering, key)
			return
		}
	}
}

// keyUp forgets key as held.
func (p *Player) keyUp(key ebiten.Key) {
	for i, k := range p.keys {
		if k == key {
			p.keys = append(p.keys[:i], p.keys[i+1:]...)
			return
		}
	}
}

// holds reports whether one of the player's bindings asks for key held down.
func (p *Player) holds(key ebiten.Key) bool {
	for _, b := range p.bindings {
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

// withHeld is mods with the last held key the player's bindings ask for, if one is down.
func (p *Player) withHeld(mods control.Mods) control.Mods {
	if len(p.keys) == 0 {
		return mods
	}
	return mods.Holding(p.keys[len(p.keys)-1])
}

// release forgets the button and reports where it went down, if it was down.
func (p *Player) release(button ebiten.MouseButton) (geom.Vec, bool) {
	at, down := p.held[button]
	delete(p.held, button)
	return at, down
}
