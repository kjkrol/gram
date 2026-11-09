package forest

import (
	"fmt"
	"time"

	"github.com/kjkrol/ceremony/act"
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

	worldPlugin *world.Plugin
	board       *board.Plugin
	types       Types
	stands      Stands
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds the forest plugin over w and the board brd, which a forest's cells and heights
// are read off.
func NewPlugin(w *world.Plugin, brd *board.Plugin) *Plugin {
	p := &Plugin{worldPlugin: w, board: brd}
	p.types.p, p.stands.p = p, p
	p.Self = world.NewSelf(w, p)
	w.Figures(trees{p: p})
	return p
}

// Types is the plugin's register of the kinds of trees.
func (p *Plugin) Types() *Types { return &p.types }

// Stands makes the roles that plant a game's forests.
func (p *Plugin) Stands() *Stands { return &p.stands }

func (p *Plugin) Name() string { return "gram.forest" }

// Install wires nothing: the trees grow and fold by the world's moments.
func (p *Plugin) Install(plugin.Installer) error { return nil }

// RunPlan does nothing: the forest has no pass of its own.
func (p *Plugin) RunPlan(goke.RunCtx, time.Duration) {}

// EventHandler is nil: the forest reads no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: a tree is an entity, saved as any, and a forest's state its effects.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

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

// the stands' mixes are checked here, the first moment every kind of tree is defined
func (t trees) Look(a *world.Atlas, _ *world.Plugin) {
	t.p.stands.check(&t.p.types)
	for _, name := range t.p.types.names {
		t.p.types.Named(name).look(a)
	}
}
