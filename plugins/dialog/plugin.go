package dialog

import (
	"fmt"
	"time"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

// The names of the plugin's effects: TalkingEf is on the speaker while it talks, and taken off
// the conversation is over; TalkedEf is on it for Config.Rest after one talked to the end.
const (
	TalkingEf = "dialog.talking"
	TalkedEf  = "dialog.talked"
)

// Config is how long a speaker rests after a conversation.
type Config struct {
	Rest time.Duration // ten seconds when zero
}

// Plugin carries conversations: the nodes a Stage defines and the Talks its entities hold.
type Plugin struct {
	*world.Self

	w          *world.Plugin
	cfg        Config
	nodes      map[uint64]*node
	changes    Changes
	conditions Conditions
	told       queues
	talking    effect.Effect
	talked     effect.Effect
	module     *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin builds the dialog plugin over w.
func NewPlugin(w *world.Plugin, cfg Config) *Plugin {
	if cfg.Rest == 0 {
		cfg.Rest = 10 * time.Second
	}
	p := &Plugin{w: w, cfg: cfg}
	p.Self = world.NewSelf(w, p)
	p.changes.p, p.conditions.p = p, p
	return p
}

func (p *Plugin) Name() string { return "gram.dialog" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = newModule(p)
	ctx.UseModule(p.module)
	return nil
}

// RunPlan carries the conversations on in the simulation. Call it after the plugins whose
// rules begin them.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// Renderer is nil: a conversation is drawn by a scene's elements (Window).
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is nil: the window's buttons give Choose.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the conversations are components, saved with their entities.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// DefineEffects defines TalkingEf and TalkedEf in the Stage's world. Call it where the Stage
// defines its effects.
func (p *Plugin) DefineEffects() {
	effects := p.w.Effects()
	effects.Define(TalkingEf, effect.Spec{effect.Described("Talking with somebody.")})
	effects.Define(TalkedEf, effect.Spec{effect.Described("Has just talked with somebody."), effect.Lasts(p.cfg.Rest)})
	p.talking, p.talked = effects.Named(TalkingEf), effects.Named(TalkedEf)
}

// Changes are what answers change in the game, by name.
func (p *Plugin) Changes() *Changes { return &p.changes }

// Conditions are when answers are offered, by name.
func (p *Plugin) Conditions() *Conditions { return &p.conditions }

// Script is the component of a kind whose conversations begin at the node called node. An
// unknown node panics.
func (p *Plugin) Script(node string) Script {
	id := idOf(node)
	if _, ok := p.nodes[id]; !ok {
		panic(fmt.Sprintf("dialog: no node is defined as %q", node))
	}
	return Script{Start: id}
}

// Window is the conversation of every entity that talks, pinned to it: the speaker as its
// title, what it says, and a button for each answer offered, giving Choose.
func (p *Plugin) Window() *ui.Element {
	if p.talking == (effect.Effect{}) {
		panic("dialog: Window before DefineEffects")
	}
	rows := []*ui.Element{ui.LabelOf(p.LineText()), ui.Blank().Size(0, 6)}
	for i := range MaxChoices {
		rows = append(rows, ui.ButtonOf(p.ChoiceText(i), Choose{Index: i}))
	}
	return ui.WindowOf(p.SpeakerText(), rows...).Under(p.talking)
}
