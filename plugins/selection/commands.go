package selection

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/uid"
)

// Select is the command to select: exactly IDs when given, else every Selectable entity in Box
// (world units); Additive keeps what was selected before.
type Select struct {
	IDs []uid.UID64
	Box geom.AABB
	// Screen is the rectangle the player picked in, in the pixels of the view Camera draws; with
	// Camera, it decides which of the entities in Box are hit by where they are drawn.
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

// Hover is the command noting the entity drawn under the cursor as Hovered, given every tick the
// cursor lies over the player's picture. A tick without one, nobody is Hovered.
type Hover struct {
	At     geom.Vec
	Screen geom.AABB
	Camera camera.Camera
}

// Allow is the command by which an entity may be selected from now on, with Selected selected at
// once too. An entity gives it itself (kind.Entry.Told); a player's goes to its selected units.
type Allow struct{ Selected bool }

// Forbid is the command by which an entity may be selected no longer, and is unselected.
type Forbid struct{}

type effectCommand struct {
	cmd     ceremony.Command
	only    tag.Tags[cond.Roles]
	pointed bool
	aimed   bool
	at      geom.Vec
	screen  geom.AABB
	camera  camera.Camera
}

func (Select) Command()        {}
func (Marquee) Command()       {}
func (Hover) Command()         {}
func (Allow) Command()         {}
func (Forbid) Command()        {}
func (effectCommand) Command() {}

var (
	_ command.Command = Select{}
	_ command.Command = Marquee{}
	_ command.Command = Hover{}
	_ command.Command = Allow{}
	_ command.Command = Forbid{}
	_ command.Command = effectCommand{}
)

// In is the command as given with the cursor where ctx has it.
func (c effectCommand) In(ctx control.Context) command.Command {
	c.aimed, c.at, c.camera = true, ctx.World(ctx.Cursor), ctx.Camera
	c.screen = control.ScreenRect(ctx.Cursor, ctx.Cursor)
	return c
}

type target struct {
	only    tag.Tags[cond.Roles]
	pointed bool
}

func (target) Target() {}

func (t target) Route(c ceremony.Command) command.Command {
	return effectCommand{cmd: c, only: t.only, pointed: t.pointed}
}

func (t target) String() string {
	if t.pointed {
		return "the one pointed at"
	}
	return "the selected"
}

// Selected is whom a command is for (ceremony.Put(haste).On(sel.Selected(hasty))): the units the
// player who gives it has selected, those playing one of roles alone when any is named.
func (p *Plugin) Selected(roles ...*ceremony.Part) ceremony.Target {
	var only tag.Tags[cond.Roles]
	for _, r := range roles {
		only = only.With(r.Tag())
	}
	return target{only: only}
}

// Pointed is whom a command is for (ceremony.Take(frozen).On(sel.Pointed())): the entity under the
// cursor as the command's key is pressed. A command given without a cursor reaches nobody.
func (p *Plugin) Pointed() ceremony.Target { return target{pointed: true} }

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are where the selection's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.selects, &p.marqueeQueue, &p.effectCmds, &p.allows, &p.forbids, &p.hovers}
}

// DefaultBindings is a left drag (a click is a drag of no length) into a Select of the box it
// drew, Shift for an additive one, the box shown as a Marquee while the button is held, C to
// follow the one unit chosen (FollowKey), and the cursor over the world into a Hover.
func (p *Plugin) DefaultBindings() []control.Binding {
	box := func(additive bool) func(c control.Context) (Select, bool) {
		return func(c control.Context) (Select, bool) {
			return Select{Box: c.WorldBox(c.Start, c.Cursor), Screen: control.ScreenRect(c.Start, c.Cursor), Camera: c.Camera, Additive: additive}, true
		}
	}
	return []control.Binding{
		control.Build(control.Drag{Button: control.MouseButtonLeft}, "Select", box(false)),
		control.Build(control.Drag{Button: control.MouseButtonLeft, Mods: control.Mods{Shift: true}}, "Add to selection", box(true)),
		control.Build(control.ButtonHeld{Button: control.MouseButtonLeft}, "Selection box", func(c control.Context) (Marquee, bool) {
			return Marquee{Screen: control.ScreenRect(c.Start, c.Cursor), Camera: c.Camera}, true
		}),
		p.FollowKey(control.KeyC),
		control.Build(control.CursorOver{}, "Point at", func(c control.Context) (Hover, bool) {
			return Hover{At: c.World(c.Cursor), Screen: control.ScreenRect(c.Cursor, c.Cursor), Camera: c.Camera}, true
		}),
	}
}

// FollowKey is key fastening the camera the player looks through over the one unit it has chosen
// (Chosen), or letting it go: a cameras.Follow.
func (p *Plugin) FollowKey(key control.Key) control.Binding {
	return control.Build(control.KeyPress{Key: key}, "Follow the selected unit, or stop", func(c control.Context) (cameras.Follow, bool) {
		id, ok := p.Chosen(c.Player)
		return cameras.Follow{Camera: c.Camera, Entity: id, On: ok}, true
	})
}
