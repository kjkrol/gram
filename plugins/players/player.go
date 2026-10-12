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
	sections section.Reader // the Stage's, telling which part is being defined; nil for none
	pic      picture        // the picture it acts through, as last wired
	wiredBy  *through       // the wire that told it of pic, and in which pass of input
	wiredIn  uint64
	local    bool
	bindings []control.Binding
	own      []bool       // whether each of bindings is the game's own: bound in its Controls
	in       input        // what the event handler has seen of its keys and buttons
	tool     control.Tool // what it holds in hand (control.Take), none for nothing
}

// Tool is what the player holds in hand: none for nothing.
func (p *Player) Tool() control.Tool { return p.tool }

// fires reports whether b fires for the player on its trigger with the camera fastened how: a
// binding of the tool it holds, or one of no tool none of those takes the trigger from.
func (p *Player) fires(b control.Binding, how camera.How) bool {
	if !b.Holds(how) || b.Tool() != "" && b.Tool() != p.tool {
		return false
	}
	if b.Tool() != "" || p.tool == "" {
		return true
	}
	for _, o := range p.bindings {
		if o.Tool() == p.tool && o.Trigger == b.Trigger && o.Holds(how) {
			return false
		}
	}
	return true
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

// Bind adds bindings to the player; two on one Trigger holding in one camera mode with one tool
// are an error, never a silent last-one-wins. Those bound in the Stage's Controls section are the
// game's own, listed apart under "Game" (Shortcuts).
func (p *Player) Bind(bindings ...control.Binding) error {
	if err := p.world.InSection("keys bound for "+p.Name, section.Players, section.Controls); err != nil {
		return err
	}
	own := p.sections != nil && p.sections.Section() == section.Controls
	for _, b := range bindings {
		if b.Type() == nil {
			return fmt.Errorf("players: %q is not a Binding made with control.Build or Give", b.Label)
		}
		for _, have := range p.bindings {
			if have.Trigger == b.Trigger && have.Overlaps(b) {
				return fmt.Errorf("players: %q and %q are both bound to %v for %s", have.Label, b.Label, b.Trigger, p.Name)
			}
		}
		p.bindings = append(p.bindings, b)
		p.own = append(p.own, own)
	}
	return nil
}

// Bindings lists what the player can do, in the order bound.
func (p *Player) Bindings() []control.Binding { return p.bindings }
