package navigation

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugin"
)

// world units under the point picked within which a deck still counts as picked
const pickReach = 1e-2

// Routes is the command showing the routes of the selected units drawn, or hiding them
// (Settings.Routes); their goals are drawn always. The settings screen alone.
type Routes struct{ Shown bool }

// Settings is what navigation lets the players adjust, saved with the game.
type Settings struct {
	Routes bool
}

// Resources is what navigation publishes: its Settings.
type Resources struct {
	Settings Settings
}

func (Routes) Command() {}

// Queues are the queues of every command the plugin carries out, for the players plugin.
func (p *Plugin) Queues() []command.CommandQueue {
	queues := []command.CommandQueue{&p.moves, &p.looks, &p.routes, &p.onwards}
	return append(queues, p.given.All()...)
}

// DefaultBindings is a right click into a MoveTo of the cell and level under the cursor, Shift
// appending, and a right drag into a LookAt at every move while the player has a selection.
func (p *Plugin) DefaultBindings() []control.Binding {
	brd := p.boardPlugin.Read(0).Board()
	to := func(appendIt bool) func(c control.Context) (MoveTo, bool) {
		return func(c control.Context) (MoveTo, bool) {
			if c.Dragged() {
				return MoveTo{}, false
			}
			at, z, picked := c.Point(c.Cursor)
			if !picked {
				cell, ok := brd.CellFor(at, f32.Inf(1), 0)
				return MoveTo{Cell: cell, At: at, Append: appendIt}, ok
			}
			cell, deck, ok := brd.LevelFor(at, z, pickReach)
			level := OnGround
			if deck {
				level = OnDeck
			}
			return MoveTo{Cell: cell, At: at, Append: appendIt, Level: level}, ok
		}
	}
	// without a selection the drag is left to whatever yields to it: the cameras' grab
	turn := func(c control.Context) (LookAt, bool) {
		return LookAt{At: c.World(c.Cursor)}, c.Dragged() && p.sel.Read(0).Any(c.Player)
	}
	return []control.Binding{
		control.Build(control.Drag{Button: control.MouseButtonRight}, "Move selected units here", to(false)),
		control.Build(control.Drag{Button: control.MouseButtonRight, Mods: control.Mods{Shift: true}}, "Add a waypoint", to(true)),
		control.Build(control.ButtonHeld{Button: control.MouseButtonRight}, "Turn selected units to look at the cursor", turn),
	}
}

// Settings is the one setting, the routes drawn (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{control.Switch{
		Label: "Routes of selected units",
		Get:   p.read.RoutesShown,
		Set:   func(on bool) command.Command { return Routes{Shown: on} },
	}}
}

var _ command.Command = Routes{}

var _ plugin.CommandHandler = (*Plugin)(nil)
