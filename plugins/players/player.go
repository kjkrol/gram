package players

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/view"
)

// Player is whoever acts in the game and may look at a part of it: a camera and the View through
// it, its part of the screen, and — at this keyboard — the bindings that turn its input into
// commands.
type Player struct {
	ID     control.PlayerID
	Name   string
	Camera camera.Camera
	View   *view.View

	world    *world.Plugin
	area     geom.AABB // its part of the screen, as last laid out; zero is all of it
	local    bool
	bindings []control.Binding
	in       input // what the event handler has seen of its keys and buttons
}

// Area is the player's part of the screen, in pixels, as the viewports last laid it out; zero
// before.
func (p *Player) Area() geom.AABB { return p.area }

// Bind adds bindings to the player; two on one Trigger holding in one camera mode are an error,
// never a silent last-one-wins.
func (p *Player) Bind(bindings ...control.Binding) error {
	if err := p.world.InSection("keys bound for "+p.Name, section.Players, section.Controls); err != nil {
		return err
	}
	for _, b := range bindings {
		if b.Command() == nil {
			return fmt.Errorf("players: %q is not a Binding built with control.Command", b.Label)
		}
		for _, have := range p.bindings {
			if have.Trigger == b.Trigger && have.Overlaps(b) {
				return fmt.Errorf("players: %q and %q are both bound to %v for %s", have.Label, b.Label, b.Trigger, p.Name)
			}
		}
		p.bindings = append(p.bindings, b)
	}
	return nil
}

// Bindings lists what the player can do, in the order bound.
func (p *Player) Bindings() []control.Binding { return p.bindings }
