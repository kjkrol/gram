package buildings

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// Raise is the command a settlement gives itself under a world.Unfold to have the buildings the
// level's file stands in the stretch unfolding spawned as its parts.
type Raise struct{ in uid.UID64 }

// Aim tells the command the stretch of the settlement unfolding.
func (r *Raise) Aim(who uid.UID64) { r.in = who }

// OpenRooms has a house a unit stands in open round its room before a camera of the tactical
// view, loose or centred on a unit, letting the cursor and the eye in, or stand whole there
// (Settings.Shut). The settings screen alone, under Game.
type OpenRooms struct{ On bool }

// OpenRoomsBehind has a house a unit stands in open round its room before a camera behind a unit,
// letting the cursor and the arm in, or stand whole there (Settings.ShutBehind). The settings
// screen alone, under Game.
type OpenRoomsBehind struct{ On bool }

func (Raise) Command()           {}
func (OpenRooms) Command()       {}
func (OpenRoomsBehind) Command() {}

var (
	_ command.Command = Raise{}
	_ host.Aimed      = (*Raise)(nil)
	_ command.Command = OpenRooms{}
	_ command.Command = OpenRoomsBehind{}
)

// Queues are the plugin's command queues: Raise, OpenRooms and OpenRoomsBehind.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.raises, &p.opened, &p.openedBehind}
}

// Settings are whether a house a unit stands in opens round its room in the tactical view and
// behind a unit, where a kind of building has a room (plugin.Tunable, under Game).
func (p *Plugin) Settings() []control.Setting {
	if !p.types.rooms {
		return nil
	}
	return []control.Setting{
		control.Switch{
			Label:   "A house a unit stands in opens round it",
			Section: control.Game,
			Get:     func() bool { return !p.Res.Settings.Shut },
			Set:     func(on bool) command.Command { return OpenRooms{On: on} },
		},
		control.Switch{
			Label:   "And behind a unit",
			Section: control.Game,
			Get:     func() bool { return !p.Res.Settings.ShutBehind },
			Set:     func(on bool) command.Command { return OpenRoomsBehind{On: on} },
		},
	}
}
