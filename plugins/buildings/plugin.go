package buildings

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"time"

	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
)

// BuildingRole is the role every building plays, the plugin's own: a building goes as its
// sector folds.
const BuildingRole = "buildings.building"

// Sky is the air between the eye and a building far off, the atmosphere's.
type Sky interface{ Air() air.Weather }

// Plugin stands the buildings of a level's file: the kinds a game defines on its Types, drawn
// as solids at every level, entities where their settlement's sectors come near.
type Plugin struct {
	*world.Self

	Res Resources

	worldPlugin *world.Plugin
	board       *board.Plugin
	sky         Sky
	types       Types
	module      *module
	solids      *solids
	write       Write

	raises command.Queue[Raise]
	kept   *plugin.Kept[Resources]
}

// Resources is the state of buildings, saved with the game whole: the buildings standing, by
// the sector their middles lie in.
type Resources struct{ Stood map[int][]Stood }

// Stood is a building standing: its kind and its line.
type Stood struct {
	Kind string
	Row  Row
}

var (
	_ plugin.Plugin         = (*Plugin)(nil)
	_ plugin.CommandHandler = (*Plugin)(nil)
	_ plugin.Restorer       = (*Plugin)(nil)
)

// NewPlugin builds the plugin of buildings over w, a world with heights, and the board brd,
// which their ground is read off.
func NewPlugin(w *world.Plugin, brd *board.Plugin) *Plugin {
	if !w.Res.Config.Heights {
		panic("buildings: the world has no heights (world.Config.Heights): a solid stands " +
			"on a ground in relief")
	}
	p := &Plugin{worldPlugin: w, board: brd, Res: Resources{Stood: map[int][]Stood{}}}
	p.kept = plugin.Keep(&p.Res)
	p.types.p, p.write.p = p, p
	p.solids = &solids{p: p}
	p.Self = world.NewSelf(w, p)
	w.Figures(own{})
	return p
}

// WithAir has the buildings far off hazed by the air of sky, as the ground is.
func (p *Plugin) WithAir(sky Sky) *Plugin {
	p.sky = sky
	return p
}

// Types is the plugin's register of the kinds of buildings.
func (p *Plugin) Types() *Types { return &p.types }

func (p *Plugin) Name() string { return "gram.buildings" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	ctx.Draws(p.solids)
	return nil
}

// RunPlan stands the buildings of the stretches told to Raise in the simulation.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the plugin's Resources, kept whole; a building raised is an entity, saved as
// any.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Resources loaded and has the buildings laid anew.
func (p *Plugin) Restore() {
	p.kept.Restore()
	if p.Res.Stood == nil {
		p.Res.Stood = map[int][]Stood{}
	}
	p.solids.stale = true
}

func (p *Plugin) DefaultBindings() []control.Binding { return nil }

func (p *Plugin) String() string { return fmt.Sprintf("buildings %v", p.types.names) }

// raise gives the settlement of each Raise the buildings standing in the stretch it was aimed
// at; a preview for the sheets, which hold no building, gets none
func (p *Plugin) raise() {
	r := p.worldPlugin.Read(0)
	carrier := p.worldPlugin.Write().Carrier()
	p.raises.Drain(func(i command.Issued[Raise]) {
		st, ok := r.Unfolding(i.Command.in)
		if !i.ByEntity || !ok || st.Preview || st.Whole != i.Entity {
			return
		}
		mid := geom.NewVec((st.Box.TopLeft.X+st.Box.BottomRight.X)/2,
			(st.Box.TopLeft.Y+st.Box.BottomRight.Y)/2)
		for _, b := range p.Res.Stood[st.Sector] {
			at := p.types.specs[b.Kind].of(b.Row).mid
			carrier.PutFrom(i.Entity, world.SpawnPartIn(i.Command.in,
				kind.EntryOf(b.Kind, b.Row), at.Sub(mid)))
		}
	})
}

// the plugin's own part of the world's figures: the buildings' role
type own struct{}

func (own) Define(*world.Plugin) {}

func (own) Look(*world.Atlas, *world.Plugin) {}

func (own) Rules(w *world.Plugin) {
	w.Roles().Define(BuildingRole, act.Branch[world.Folding]{
		Name: "fold in",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(world.Despawn{}),
	})
}
