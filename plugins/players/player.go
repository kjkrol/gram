package players

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/world"
)

// Player is whoever acts in the game: the picture of the world it acts through — wired by the
// scene shown (Plugin.Through), none in a Stage without one — and, at this keyboard, the bindings
// that turn its input into commands.
type Player struct {
	ID   control.PlayerID
	Name string

	world    *world.Plugin
	pic      picture  // the picture it acts through, as last wired
	wiredBy  *through // the wire that told it of pic, and in which pass of input
	wiredIn  uint64
	local    bool
	bindings []control.Binding
	in       input // what the event handler has seen of its keys and buttons
}

// picture is a picture of the world a player acts through: where it lies on the screen — zero is
// all of it — and the camera it is drawn through, nil for none.
type picture struct {
	area   geom.AABB
	camera camera.Camera
}

// Area is the player's part of the screen, in pixels, as its picture was last laid out; zero
// before.
func (p *Player) Area() geom.AABB { return p.pic.area }

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
