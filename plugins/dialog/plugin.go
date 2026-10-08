package dialog

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/ui"
)

// The names of the plugin's effects, the handles of every conversation (DefineEffects).
const (
	TalkingEf = "dialog.talking" // on the speaker while it talks; taken off, the conversation is over
	TalkedEf  = "dialog.talked"  // on the speaker for Config.Rest after a conversation talked to the end
)

// Config is how the plugin weighs moods and how long a speaker rests after a conversation.
type Config struct {
	Friend int8          // the mood from which the speaker is a friend; 30 when zero
	Enemy  int8          // the mood up to which it is an enemy; -30 when zero
	Rest   time.Duration // how long TalkedEf lasts; ten seconds when zero
}

// Plugin carries conversations: the nodes a Stage defines (Define, Load), the conversations its
// entities hold (Talk) and what they make of whom they talked with (Memory).
type Plugin struct {
	*world.Self // its own entity: its knobs, the roles it plays, the effects it is under

	w       *world.Plugin
	cfg     Config
	nodes   map[uint64]*node
	told    queues
	talking effect.Effect
	talked  effect.Effect
	module  *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin builds the dialog plugin over w; hand it to the players plugin for its commands.
func NewPlugin(w *world.Plugin, cfg Config) *Plugin {
	if cfg.Friend == 0 {
		cfg.Friend = 30
	}
	if cfg.Enemy == 0 {
		cfg.Enemy = -30
	}
	if cfg.Rest == 0 {
		cfg.Rest = 10 * time.Second
	}
	return &Plugin{Self: world.NewSelf(w, "gram.dialog"), w: w, cfg: cfg}
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.dialog" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = newModule(p)
	ctx.UseModule(p.module)
	return nil
}

// RunPlan carries the conversations on in the simulation; call it after the plugins whose rules
// begin them.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op: a conversation is drawn by a scene's elements (Window).
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is nil: a conversation is drawn by a scene's elements (Window).
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is nil: the window's buttons give Choose.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the conversations are components, saved with their entities.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// =================================================================
// dialog-specific
// =================================================================

// DefineEffects defines TalkingEf and TalkedEf in the Stage's world: call it where the Stage
// defines its effects.
func (p *Plugin) DefineEffects() {
	effects := p.w.Effects()
	effects.Define(TalkingEf, effect.Spec{effect.Described("Talking with somebody.")})
	effects.Define(TalkedEf, effect.Spec{effect.Described("Has just talked with somebody."), effect.Lasts(p.cfg.Rest)})
	p.talking, p.talked = effects.Named(TalkingEf), effects.Named(TalkedEf)
}

// Script is where the conversations of a kind's entities begin: the node called node — a kind's
// component, comp.Const(p.Script(node)) for all of them, or each one's own through comp.Load. An
// unknown node panics.
func (p *Plugin) Script(node string) Script {
	id := idOf(node)
	if _, ok := p.nodes[id]; !ok {
		panic(fmt.Sprintf("dialog: no node is defined as %q", node))
	}
	return Script{Start: id}
}

// Window is the conversation of every entity that talks, pinned to it (Under TalkingEf): the
// speaker as its title, what it says, and a button for each answer offered, giving Choose. A game
// may lay its own out of SpeakerText, LineText and ChoiceText.
func (p *Plugin) Window() *ui.Element {
	if p.talking == (effect.Effect{}) {
		panic("dialog: Window before DefineEffects")
	}
	rows := []*ui.Element{ui.LabelOf(p.LineText())}
	for i := range MaxChoices {
		rows = append(rows, ui.ButtonOf(p.ChoiceText(i), Choose{Index: i}))
	}
	return ui.WindowOf(p.SpeakerText(), rows...).Under(p.talking)
}

// stance is the word for mood.
func (p *Plugin) stance(mood int8) string {
	switch {
	case mood >= p.cfg.Friend:
		return "Friend"
	case mood <= p.cfg.Enemy:
		return "Enemy"
	}
	return "Neutral"
}
