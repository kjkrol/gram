package selection

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Select is the command to select: exactly IDs when given, else every Selectable entity in Box
// (world units); Additive keeps what was selected before.
type Select struct {
	IDs []uid.UID64
	Box geom.AABB
	// Screen is the rectangle the player picked in, in the pixels of the view Camera draws; set with
	// Camera, it decides which of the entities in Box are hit by where they are drawn — a unit
	// standing high or flying is where it is seen.
	Screen   geom.AABB
	Camera   camera.Camera
	Additive bool
}

// Marquee is the box of a selection being dragged, in the pixels of the view Camera draws: shown
// until the Select that ends it.
type Marquee struct {
	Screen geom.AABB
	Camera camera.Camera
}

// Follow is the command to follow the one selected unit with Camera, or to stop following.
type Follow struct{ Camera camera.Camera }

// Apply is the command to put Effect on every Selected unit of the player who gives it, as its
// Spec says: an ability — a sprint, a spell — which rules and knobs carry on from. Only, unless
// empty, narrows it to the units playing one of those roles (rule.Plays).
type Apply struct {
	Effect effect.Effect
	Only   tag.Tags[rule.Roles]
}

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are where Select, Follow and Apply land — for the players plugin.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.selects, &p.marqueeQueue, &p.follows, &p.applies}
}

// Abilities are the bindings of what the roles can do (rule.Role.Can): each its trigger into an
// Apply of its effect, Only to the units playing its role, under its label — for a player's Bind.
func (p *Plugin) Abilities(roles ...*rule.Role) []control.Binding {
	var bindings []control.Binding
	for _, r := range roles {
		for _, a := range r.Abilities() {
			apply := Apply{Effect: a.Effect, Only: tag.Tags[rule.Roles](0).With(a.Role.Tag())}
			bindings = append(bindings, control.Command(a.Trigger, a.Label, func(control.Context) (Apply, bool) { return apply, true }))
		}
	}
	return bindings
}

// DefaultBindings is a left drag (a click is a drag of no length) into a Select of the box it
// drew, Shift for an additive one, the box shown as a Marquee while the button is held, and F to
// follow the one selected unit or stop following.
func (p *Plugin) DefaultBindings() []control.Binding {
	box := func(additive bool) func(c control.Context) (Select, bool) {
		return func(c control.Context) (Select, bool) {
			return Select{Box: c.WorldBox(c.Start, c.Cursor), Screen: control.ScreenRect(c.Start, c.Cursor), Camera: c.Camera, Additive: additive}, true
		}
	}
	return []control.Binding{
		control.Command(control.Drag{Button: control.MouseButtonLeft}, "Select", box(false)),
		control.Command(control.Drag{Button: control.MouseButtonLeft, Mods: control.Mods{Shift: true}}, "Add to selection", box(true)),
		control.Command(control.ButtonHeld{Button: control.MouseButtonLeft}, "Selection box", func(c control.Context) (Marquee, bool) {
			return Marquee{Screen: control.ScreenRect(c.Start, c.Cursor), Camera: c.Camera}, true
		}),
		control.Command(control.KeyPress{Key: control.KeyC}, "Follow the selected unit", func(c control.Context) (Follow, bool) {
			return Follow{Camera: c.Camera}, true
		}),
	}
}
