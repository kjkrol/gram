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
// for the state it saves. A
// [Builtin] plugin is one the engine installs itself, such as the world; Use refuses it.
//
// # Installer
//
// [Installer] is ECS wiring — UseModule, Setup, RegSys, ECS — and Hosts, which tells the engine
// the hosts of the rules of the moments the plugin catches. Install only queues;
// the engine flushes every plugin's wiring in one ecs.Setup after the Stage's Init. Cross-plugin
// data comes from constructor injection, not from the Installer.
//
// # Rules
//
// A plugin hosts the rules of the moments it catches in its own pass, written with package rule
// (rule.Then) and obeyed by roles. It has no way in for a game's rules: in Install it tells the
// engine its hosts (Installer.Hosts; a [Host] is a Rules, a PairRules or a StepRules), and once
// the Stage's Init returns the engine hands the rules of every role somebody plays to the hosts,
// in the order their plugins were Used, each rule to the first that does not refuse it with
// [ErrUnhosted]; any other error — [ErrHostBuilt] for one handed over after ecs.Setup — stops
// the Stage. A rule none takes is ErrUnhosted, named by its String: "fall in" of unit.Standing,
// for the role mortal. A plugin's own rules (navigation's crowd) it adds to its host itself.
//
// It runs them inside that pass: [Rules] over the entities it walks (Bind adds the columns the
// rules read to its query, Run or RunWhere over each chunk; [Own] shares a column it reads itself),
// [PairRules] over pairs (Bind its tag families to its queries, read an entity's [Marks] with
// InChunk or At, then Dispatch, DispatchEitherWay or DispatchGrouped). A rule a role narrows runs
// only where the pair's own entity plays the role too, its family among the eight a PairRules
// reads; DispatchEitherWay runs a rule with one tag on both sides at most once a pair. [StepRules]
// run once a step, walking no entities: they take a rule of rule.All alone, refusing a filtered or
// narrowed one with ErrUnhosted, and Bind, in the system's Init, marks them built. Each hands its
// rules a [Tick], made by the world's [TickSource]: the command buffer, the carrier, the game time,
// the world's seed and own entity, the places round a [Placed] moment, the wire an entity is wired
// to (Tick.Wires) and the roles it plays (Tick.Roles, for a rule's Playing). A moment is [About]
// one entity, [Met] others too, a [Subject] names another, an [Aimed] command is told whom it is
// about.
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
