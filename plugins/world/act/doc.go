// Package act is how entities behave, in one vocabulary of nodes: a trigger is a moment a plugin
// catches and what is done then, a tree is how a kind deliberates over time, an effect is a change
// that holds, a command is what an entity has done, and a fact is what a plugin tells an entity.
// The world makes and runs the trees (plugins/world), in every step of its simulation, after its
// decision systems: they stand in the tactical pause, go with the tempo, and are saved with the
// game. The effects live in the subpackage effect.
//
// Nodes are made by the methods of a builder: a [Branch] for a tree, a [Reaction] for a trigger.
// The constructor names and opens it, the methods make the nodes under it, and Do closes it round
// its body:
//
//	c := act.When[navigation.Blocked]("when blocked")
//	hold := c.Issue(navigation.Hold{}).Until(navigation.Blocked.WaitedLong)
//	return c.Do(c.First(c.If(navigation.Blocked.GivesWayFirst, hold), …))
//
// # Triggers
//
// [Trigger] is a [Reaction] of a moment a host plugin catches in its own pass — a board.Standing,
// a vision.Sighting, a collision.Meeting, a world.Moving, an effect.Idling, a clock.Moment — whose
// Do hands the host's Hook a plugin.Trigger: board.Plugin.Hook, vision's, collision's, world's.
// Self, Other and Having narrow whom it fires for; with none it fires for every entity the host
// shows it, or every pair for a moment that is [Met]. A Reaction makes [Instant] nodes alone —
// First, Then, If on the moment, Unless, IfUnder, Apply, While, Issue, ToOther, Run, RunOn — each
// done within the host's pass: a node that lasts is not to be had, the compiler says so. Runs and
// RunsOn are a trigger of a function alone. A trigger keeps no memory of its own: an effect's
// presence is its memory — "at most once a while" is Unless an effect that lasts that while. A
// moment is [About] one entity, whom the nodes act for; a clock.Moment is of none, and nodes
// acting on one fail on it.
//
// # Trees
//
// A tree is plain Go: functions returning nodes, each branch named and short, so a game takes a
// plugin's branches one by one, or composes its own. [When] opens a branch that runs while the
// entity carries a fact, [On] one that runs from the tick a fact comes until its body is done,
// [Named] one that just runs — a tree's root. A Branch's First runs its nodes in order every tick
// and stands as the first that does not fail — an earlier branch, once it can, takes over and the
// later one is stopped (a reactive selector); Then runs its nodes one after another (a sequence
// that remembers its step); If runs a node while a fact holds as a function of it says; Until waits
// for a fact to come afresh; Wait waits; Timeout, Cooldown and Invert decorate; Idle runs for ever.
// [Tree] is the component a kind gives: units.Define(..., act.Tree(navigation.Courteous())). The
// root's name is what a save knows the tree by; one name is one tree, registered again it must be
// alike. A tree holds at most [MaxNodes] nodes; its state is the entity's [Mind] — the tree, the
// nodes running, each node's step and start on the world's clock.
//
// # Effects and commands
//
// Apply casts an effect (effect.Effect) on the entity, lasting as its Spec says; While holds one
// for as long as its branch runs — in a trigger, as long as the trigger keeps firing it; Unless and
// IfUnder run a node as the entity is under one or not. Issue gives a command for the entity —
// the same command a player gives, queued for the plugin that handles its type
// (navigation.MoveTo, world.Despawn) — and does well at once: fire and forget. In a tree it hands
// back a [Command]: its Until waits for what comes of it, a fact — navigation.Arrived — and its
// Stay keeps the branch, so that a reactive branch gives it once, not every tick; put the reaction
// to what came of it earlier in the First. A command that is [Aimed] is told, as it is given, the
// [Subject] of the fact it stands under: whom the entity was blocked by, who asked. The world
// carries the commands (world.Plugin.Carry): the engine hands it every plugin.CommandHandler a
// stage uses.
//
// # Facts
//
// A fact is a component a plugin writes for entities with a Mind alone — navigation.Blocked,
// navigation.Room — and the tree only reads, so the rest pay nothing. Every fact is a component
// type, and goke registers 128 at most in all.
//
// # Conversation
//
// A Branch's Ask asks the subject of the fact it stands under for something — a type naming the
// ask, such as navigation.MakeWay — and waits for the answer: yes runs its agreed branch, no or no
// answer in time its refused one. The ask reaches the other entity a tick later as the fact
// [Asked], which its own tree answers with Agree or Refuse, or passes on with Relay to the subject
// of the fact it stands under — someone beside — telling the asker to wait ([Relayed]). The answer
// comes back as the fact [Replied]. Guards hold for every conversation: an ask reaches only an
// entity with a Mind; an ask or an answer nobody takes up is dropped after [AskLife]; a [Chain]
// lists who an ask passed through, at most [MaxChain], and a Relay to anyone on it, to the asker
// or to itself fails — so no ask goes round, and every conversation ends.
package act
