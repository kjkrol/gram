package ui

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// It is whom a command is for in an element pinned to an entity: the entity the element is
// shown for. A rule.Command for It is defined in the register as any other.
var It rule.Target = it{}

type it struct{}

func (it) Target() {}

// About is a plugin's own command that is for the entity the element giving it is pinned to,
// as a rule.Command for It is.
type About interface {
	control.Command
	About(of uid.UID64) control.Command
}

func about(cmd control.Command, id uid.UID64, pinned bool) (control.Command, bool) {
	if a, ok := cmd.(About); ok {
		if !pinned {
			return nil, false
		}
		return a.About(id), true
	}
	c, ok := cmd.(rule.Command)
	if !ok {
		return cmd, true
	}
	if _, isIt := c.Whom.(it); !isIt {
		return cmd, true
	}
	if !pinned {
		return nil, false
	}
	c.Whom = entity.ID(id)
	return c, true
}

// Show shows the scene's elements called Name.
type Show struct{ Name string }

// Hide hides the scene's elements called Name.
type Hide struct{ Name string }

// Toggle shows the scene's elements called Name where any is hidden, else hides them.
type Toggle struct{ Name string }

func (Show) Command()   {}
func (Hide) Command()   {}
func (Toggle) Command() {}

var (
	_ control.Command = Show{}
	_ control.Command = Hide{}
	_ control.Command = Toggle{}
)

func (s *Scene) carry(cmd control.Command) bool {
	switch c := cmd.(type) {
	case Show:
		s.Show(c.Name)
	case Hide:
		s.Hide(c.Name)
	case Toggle:
		s.Toggle(c.Name)
	default:
		return false
	}
	return true
}
