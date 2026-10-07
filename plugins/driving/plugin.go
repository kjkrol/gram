package driving

import (
	"fmt"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Keeping is who else may stand in a driven unit's way, and whom something else steers: the
// navigation's spacing, which navigation hands over as it is made (WithKeeping).
type Keeping interface {
	// MayStep reports whether the unit id, standing at pos in its cell at, moving in domain, may
	// walk on into the cell ahead along heading.
	MayStep(id uid.UID64, pos world.Position, at, ahead cell.ID, domain cell.Domain, heading geom.Vec) bool
	// Ordered reports whether id is steered along a route of its own: with no hand on it, the
	// driving leaves it be.
	Ordered(id uid.UID64) bool
}

// Plugin drives units by hand: the commands Ahead, Back, Turn and Toward, summed a tick into each
// hand, put on the units the hand is on and carried out — turning them, walking them on, braking
// and backing them away, flying them — over the ground of the board it is given, if any.
type Plugin struct {
	*world.Self // its own entity: its knobs, the roles it plays, the effects it is under

	worldPlugin *world.Plugin
	selected    tag.Tag[selection.Family]
	hands       hands
	board       *board.Plugin // the ground the units walk on; nil, none
	keeping     Keeping       // who else is in the way; nil, nobody
	module      *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin builds the driving over worldPlugin's units, a player's hand on those it has selected
// in selectionPlugin while its camera is fastened to none.
func NewPlugin(worldPlugin *world.Plugin, selectionPlugin *selection.Plugin) *Plugin {
	kind.Require[steering.Steering](&worldPlugin.Roster().Unit, "driving", "the profile it is driven by")
	if t := worldPlugin.Kinds().DefineTag[States](drivingName); t != Driving {
		panic(fmt.Sprintf("driving: its markers have tags of their own before %q", drivingName))
	}
	worldPlugin.Roster().Unit.Default(comp.Marks[States]())
	return &Plugin{Self: world.NewSelf(worldPlugin, "gram.driving"), worldPlugin: worldPlugin, selected: selectionPlugin.Tags().Selected}
}

// WithGround has the driven units walk the board: never onto ground their domain may not stand
// on, their cell (unit.At) and their hold on the occupancy following them cell by cell. Call
// before Use.
func (p *Plugin) WithGround(b *board.Plugin) *Plugin {
	p.board = b
	return p
}

// WithKeeping has k say who else is in a driven unit's way and whom it steers itself — a plugin
// that keeps units apart calls it as it is made (navigation.NewPlugin). Call before Use.
func (p *Plugin) WithKeeping(k Keeping) *Plugin {
	p.keeping = k
	return p
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.driving" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	shared := &units{}
	drive := &driveSystem{keeping: p.keeping, units: shared}
	if p.board != nil {
		drive.board, drive.occupancy = p.board.Res.Logic.Board, p.board.Occupancy()
	}
	p.module = &module{
		hand:  &handSystem{hands: &p.hands, selected: p.selected, units: shared},
		drive: drive,
		clock: p.worldPlugin.Clock(),
	}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan puts this tick's hands on the units at once and carries them out in the simulation; call
// it after navigation's, whose orders a hand ends.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op — the driving draws nothing.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is nil — the driving draws nothing.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is nil — a player's bindings (Keys) issue the commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil — the driving keeps nothing but the units' own components.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
