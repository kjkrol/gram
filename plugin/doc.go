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
// pass, written with package rule (rule.On), a role's rules among them. Its Hook takes them before
// or after Use until the Stage's ecs.Setup builds its systems, and refuses a rule of a moment it
// does not catch with an error wrapping [ErrUnhosted], one too late with [ErrHostBuilt]. A Stage
// hands its rules to game.Initializer.Hook, which tries the plugins in use in the order they were
// Used and hooks each rule on the first that does not refuse it with ErrUnhosted: a Hook wraps
// ErrUnhosted for that alone, as any other error stops the Stage's Hook at once. The errors name
// the rule by its String — "fall in" of unit.Standing, for the role mortal — and a plugin's Hook
// adds its Name and the moments it does take (world: Moving, Leaving or clock.Moment).
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
