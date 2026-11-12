package forest

import (
	"fmt"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
)

// TreeRole is the role every tree plays, the plugin's own: a tree goes as its sector folds.
const TreeRole = "forest.tree"

// Plugin plants forests over the board: the kinds of trees a game defines on its Types, planted
// by the rules its Stands make as a forest's sectors come near and gone as they fold.
type Plugin struct {
	*world.Self

	Res Resources

	worldPlugin *world.Plugin
	board       *board.Plugin
	types       Types
	stands      Stands
	plants      command.Queue[Plant]
	module      *module
	write       Write
	kept        *plugin.Kept[Resources]
}

// Resources is the state of the forest, saved with the game whole: the trees standing by
// themselves, by the sector each stands in.
type Resources struct{ Planted map[int][]Planted }

// Planted is a tree standing by itself: its kind and where it stands.
type Planted struct {
	Kind string
	At   geom.Vec
}

var (
	_ plugin.Plugin         = (*Plugin)(nil)
	_ plugin.CommandHandler = (*Plugin)(nil)
	_ plugin.Restorer       = (*Plugin)(nil)
)

// NewPlugin builds the forest plugin over w and the board brd, which a forest's cells and heights
// are read off.
func NewPlugin(w *world.Plugin, brd *board.Plugin) *Plugin {
	p := &Plugin{worldPlugin: w, board: brd, Res: Resources{Planted: map[int][]Planted{}}}
	p.kept = plugin.Keep(&p.Res)
	p.types.p, p.stands.p, p.write.p = p, p, p
	p.Self = world.NewSelf(w, p)
	w.Figures(trees{p: p})
	return p
}

// Types is the plugin's register of the kinds of trees.
func (p *Plugin) Types() *Types { return &p.types }

// Stands makes the roles that plant a game's forests.
func (p *Plugin) Stands() *Stands { return &p.stands }

func (p *Plugin) Name() string { return "gram.forest" }

// Install wires the pass that plants the trees a level's file stands by themselves.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan plants the trees of the stretches told to Plant in the simulation.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// EventHandler is nil: the forest reads no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the plugin's Resources, kept whole; a tree planted is an entity, saved as
// any, and a forest's state its effects.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Resources loaded.
func (p *Plugin) Restore() {
	p.kept.Restore()
	if p.Res.Planted == nil {
		p.Res.Planted = map[int][]Planted{}
	}
}

func (p *Plugin) String() string { return fmt.Sprintf("forest %v", p.types.names) }

// the plugin's own part of the world's figures: the trees' role, and each kind of tree's look
type trees struct{ p *Plugin }

func (trees) Define(*world.Plugin) {}

func (trees) Rules(w *world.Plugin) {
	w.Roles().Define(TreeRole, act.Branch[world.Folding]{
		Name: "fold in",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(world.Despawn{}),
	})
}

// the stands' mixes and the entries asked are checked here, the first moment every kind of
// tree is defined
func (t trees) Look(a *world.Atlas, _ *world.Plugin) {
	t.p.stands.check(&t.p.types)
	t.p.types.check()
	for _, name := range t.p.types.names {
		t.p.types.Named(name).look(a)
	}
}
