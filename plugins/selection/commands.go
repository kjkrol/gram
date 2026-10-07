package selection

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/rule"
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

// Allow is the command by which an entity may be selected from now on — Selected, selected at
// once too: one gives it itself as it is made (kind.Entry.Told); a player's goes to the units it
// has selected.
type Allow struct{ Selected bool }

// Forbid is the command by which an entity may be selected no longer, and is unselected: a unit
// under construction, one carried off.
type Forbid struct{}

// effectCommand is a rule.Command for the selected units, or for the one pointed at, as the selection
// carries it out: what Selected and Pointed route a command to.
type effectCommand struct {
	cmd     rule.Command
	only    tag.Tags[rule.Roles] // the selected playing one of these; empty: every one
	pointed bool
	aimed   bool // In told where the cursor was
	at      geom.Vec
	screen  geom.AABB
	camera  camera.Camera
}

// In is the command as given with the cursor where c has it: whom Pointed means.
func (c effectCommand) In(ctx control.Context) any {
	c.aimed, c.at, c.camera = true, ctx.World(ctx.Cursor), ctx.Camera
	c.screen = control.ScreenRect(ctx.Cursor, ctx.Cursor)
	return c
}

// target is whom a command is for, as the selection knows them.
type target struct {
	only    tag.Tags[rule.Roles]
	pointed bool
}

func (target) Target() {}

func (t target) Route(c rule.Command) any {
	return effectCommand{cmd: c, only: t.only, pointed: t.pointed}
}

func (t target) String() string {
	if t.pointed {
		return "the one pointed at"
	}
	return "the selected"
}

// Selected is whom a command is for — rule.Cast(haste).On(sel.Selected(hasty)): the units the
// player who gives it has selected, those playing one of roles alone when any is named.
func (p *Plugin) Selected(roles ...*rule.Part) rule.Target {
	var only tag.Tags[rule.Roles]
	for _, r := range roles {
		only = only.With(r.Tag())
	}
	return target{only: only}
}

// Pointed is whom a command is for — rule.Lift(frozen).On(sel.Pointed()): the entity under the
// cursor as the command's key is pressed, the nearest of those drawn there; nobody for a command
// given some other way.
func (p *Plugin) Pointed() rule.Target { return target{pointed: true} }

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are where Select, Allow, Forbid and the commands for the selected and the pointed at
// land — for the players plugin.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.selects, &p.marqueeQueue, &p.effectCmds, &p.allows, &p.forbids}
}

// DefaultBindings is a left drag (a click is a drag of no length) into a Select of the box it
// drew, Shift for an additive one, the box shown as a Marquee while the button is held, and C to
// follow the one unit chosen (FollowKey).
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
		p.FollowKey(control.KeyC),
	}
}

// FollowKey is key fastening the camera the player looks through over the one unit it has chosen
// (Chosen) — kept in the middle of the screen as it goes — or letting it go: a cameras.Follow.
func (p *Plugin) FollowKey(key control.Key) control.Binding {
	return control.Command(control.KeyPress{Key: key}, "Follow the selected unit, or stop", func(c control.Context) (cameras.Follow, bool) {
		id, ok := p.Chosen(c.Player)
		return cameras.Follow{Camera: c.Camera, Entity: id, On: ok}, true
	})
}
