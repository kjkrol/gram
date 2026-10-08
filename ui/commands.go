package ui

import (
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// It is whom a command is for in an element pinned to an entity: that entity —
// rule.Lift(greeting).On(ui.It), defined in the register as any command. A button of the element
// shown for an entity gives it for that entity (entity.ID).
var It rule.Target = it{}

type it struct{}

func (it) Target() {}

// About is a command of a plugin's own that is for the entity the element giving it is pinned
// to, as a rule.Command for It is: About is the command for that entity — dialog.Choose, the
// answer to whoever the window stands above.
type About interface{ About(of uid.UID64) any }

// about is cmd for the entity id where it is for It or an About: false where it is and no entity
// is pinned.
func about(cmd any, id uid.UID64, pinned bool) (any, bool) {
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

// carry carries out cmd when it is one of the scene's own, reporting whether it was.
func (s *Scene) carry(cmd any) bool {
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
