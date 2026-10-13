package players

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/world"
)

// Player is whoever acts in the game: the picture of the world it acts through and, at this
// keyboard, the bindings that turn its input into commands.
type Player struct {
	ID   command.Issuer
	Name string

	world    *world.Plugin
	sections section.Reader
	pic      picture
	wiredBy  *through
	wiredIn  uint64
	local    bool
	bindings []control.Binding
	own      []bool
	in       input
	tool     control.Tool
}

// Tool is what the player holds in hand: none for nothing.
func (p *Player) Tool() control.Tool { return p.tool }

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

type picture struct {
	area   geom.AABB
	camera camera.Camera
}

// Area is the player's part of the screen, in pixels, as its picture was last laid out.
func (p *Player) Area() geom.AABB { return p.pic.area }

// Bind adds bindings to the player; two on one Trigger holding in one camera mode with one tool
// are an error. Those bound in the Stage's Controls section are the game's own, listed apart
// under "Game".
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
