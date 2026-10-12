// Package plugin is the extension contract: what a [Plugin] is, what it is handed at install
// time, and how a game's rules are hosted inside a plugin's own pass. Built-in plugins and
// third-party ones implement the same interface.
//
// # Plugin
//
// A [Plugin] has a Name (saves match its state by it), an Install that queues its ECS wiring, a
// RunPlan the Stage calls once a tick, and optional faces: WithRenderer and Renderer for what it
// draws, EventHandler for the input it reads, Serializable for the state it saves. A [Builtin]
// plugin is one the engine installs itself. A plugin tells the Stage of itself as it is made
// ([Made]); the Stage uses the plugins in the order made.
//
// # Installer
//
// [Installer] is ECS wiring and Hosts, which tells the engine the hosts of the rules of the
// moments the plugin catches. Install only queues; the engine flushes every plugin's wiring in
// one ecs.Setup after the Stage's Init. What a plugin needs of another it takes in its
// constructor.
//
// # Rules
//
// A plugin hosts the rules of the moments it catches in its own pass: in Install it tells the
// engine its hosts (a [Host] is a [Rules], a [PairRules] or a [StepRules]), and once the Stage's
// Init returns the engine hands the rules of every role somebody plays to the first host that
// does not refuse them with [ErrUnhosted]; [ErrHostBuilt] is a rule handed over after ecs.Setup.
// Rules run over the entities the plugin walks (Bind, Run, RunWhere; [Own] shares a column it
// reads itself), PairRules over pairs (Bind, [Marks] read with InChunk or At, then Dispatch,
// DispatchEitherWay or DispatchGrouped), StepRules once a step, walking no entities. Each hands
// its rules a [Tick], made by the world's [TickSource]. A moment is [About] one entity, [Met]
// others too, a [Subject] names another, an [Aimed] command is told whom it is about, a [Placed]
// moment stands on places of their own.
//
// # Commands
//
// What a player wants goes the other way, as a command of package control. A [CommandHandler]
// defines command types and carries them out: it keeps a control.Queue of each, lists them in
// Queues, drains them in its own pass, and suggests the control.Bindings that issue them. A
// command type has one handler.
//
// # Optional interfaces
//
// [Serializable] gives Persistence what to encode and decode, [Populator] seeds initial state in
// a Stage entered afresh, [Restorer] is called right after its state was decoded, [PostLoader]
// supplies a one-time system run after the entities were restored.
package plugin
