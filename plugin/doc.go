// Package plugin is the extension contract: what a [Plugin] is, what it is handed at install
// time, and how game logic is hosted inside a plugin's own pass as a rule. Built-in plugins
// and third-party ones implement exactly the same interface.
//
// # Plugin
//
// A [Plugin] has a Name (saves match its state by it, and Use rejects a duplicate), an Install
// that queues its ECS wiring, a RunPlan the game calls once a tick in the order it needs, and
// optional faces: WithRenderer and Renderer for what it draws, EventHandler for the input it
// reads (the players plugin's, in practice: other plugins take commands, not input), Serializable
// for the state it saves, Hook for the rules it hosts. A
// [Builtin] plugin is one the engine installs itself, such as the world; Use refuses it.
//
// # Installer
//
// [Installer] is ECS wiring and nothing else: UseModule, Setup, RegSys, ECS. Install only queues;
// the engine flushes every plugin's wiring in one ecs.Setup after the Stage's Init. Cross-plugin
// data comes from constructor injection, not from the Installer.
//
// # Rules
//
// A plugin hosts the rules a game hooks on it (Hook): rules of the moments it catches in its own
// pass, written with package rule (rule.On) and run by its hosts (rule.EachHost, rule.PairHost,
// rule.ListHost) inside that pass. Package rule holds the rule, the hosts and the Tick a host hands
// its rules. Hook before Use.
//
// # Commands
//
// What a player wants goes the other way, as a command — the vocabulary is package control's. A
// [CommandHandler] is a plugin, or a game, that defines command types and carries them out: it
// keeps a control.Queue of each as a field, lists them in Queues, drains them in its own pass, and
// suggests the control.Bindings that issue them. A command type has one handler — a subscriber is
// what hears an event, and there may be many. The players plugin is the carrier built over the
// handlers for the players; the world carries the commands its entities give themselves
// (Order in a rule or a plan), the engine handing it every handler a stage uses.
//
// # Optional interfaces
//
// [Serializable] contributes pointers for Persistence to encode and decode. [Populator] seeds its
// own initial state, run only when a Stage starts without a restored save. [Restorer] is called
// right after its state was decoded; [PostLoader] supplies a one-time system run after the
// entities were restored, for a hook that must query them.
package plugin
