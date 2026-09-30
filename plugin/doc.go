// Package plugin is the extension contract: what a [Plugin] is, what it is handed at install
// time, and how game logic is hosted inside a plugin's own pass as a trigger. Built-in plugins
// and third-party ones implement exactly the same interface.
//
// # Plugin
//
// A [Plugin] has a Name (saves match its state by it, and Use rejects a duplicate), an Install
// that queues its ECS wiring, a RunPlan the game calls once a tick in the order it needs, and
// optional faces: WithRenderer and Renderer for what it draws, EventHandler for the input it
// reads (the players plugin's, in practice: other plugins take commands, not input), Serializable
// for the state it saves, Hook for the triggers it hosts. A
// [Builtin] plugin is one the engine installs itself, such as the world; Use refuses it.
//
// # Installer
//
// [Installer] is ECS wiring and nothing else: UseModule, Setup, RegSys, ECS. Install only queues;
// the engine flushes every plugin's wiring in one ecs.Setup after the Stage's Init. Cross-plugin
// data comes from constructor injection, not from the Installer.
//
// # Triggers
//
// A [Trigger] is a moment a plugin catches in its own pass and what is done then, built with
// act.Trigger and hooked with the plugin's Hook: act.Trigger[vision.Sighting](name).Self(a).
// Other(b).Do(body) fires for every observer carrying tag a and what it sees carrying b;
// act.Trigger[board.Standing](name).Do(body) for every entity on the board. The
// payload type — a Sighting, a Standing, a Moving — is what says which plugin hosts it; a host
// refuses another's with [ErrUnhosted], and one hooked after the host's queries were built with
// [ErrHostBuilt]. Hook before Use. The hosts behind triggers are in plugin/host, a plugin author's
// package. A trigger is instant, in the host's pass; what lasts over ticks and talks to other
// entities is a kind's tree (plugins/world/act).
//
// A tag is a bit of a family (plugins/world/entity/tag). The families a host's triggers name join
// its queries as optional components, so a trigger costs no query of its own, and a host reads
// what an entity carries as [Marks] — what a payload passes on for [Marks.Carries]. One host's
// triggers may name at most [MaxFamilies] families.
//
// # Commands
//
// What a player wants goes the other way, as a command — the vocabulary is package control's. A
// [CommandHandler] is a plugin, or a game, that defines command types and carries them out: it
// keeps a control.Queue of each as a field, lists them in Queues, drains them in its own pass, and
// suggests the control.Bindings that issue them. A command type has one handler — a subscriber is
// what hears an event, and there may be many. The players plugin is the carrier built over the
// handlers for the players; the world carries the commands its entities give themselves
// (Issue in an act tree or trigger), the engine handing it every handler a stage uses.
//
// # Tick
//
// [Tick] is what a trigger is told about the pass it runs in: the command buffer its structural
// changes go through (they land when the pass is over), the time read once for the whole pass,
// the tick's length, and the world's carrier of commands.
//
// # Optional interfaces
//
// [Serializable] contributes pointers for Persistence to encode and decode. [Populator] seeds its
// own initial state, run only when a Stage starts without a restored save. [Restorer] is called
// right after its state was decoded; [PostLoader] supplies a one-time system run after the
// entities were restored, for a hook that must query them.
package plugin
