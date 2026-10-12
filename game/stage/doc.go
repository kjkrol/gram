// Package stage defines a game.Stage a section at a time, always in the same order: a chain
// begun with [New] and ended with Update, each link one part of the game and nothing else.
//
//	mainStage := stage.New(MeadowStage).
//		World(worldCfg).
//		Plugins(a.makePlugins).
//		Players(a.definePlayer).
//		Effects(a.defineEffects).
//		Rules(a.defineRules).
//		Commands(a.defineCommands).
//		With(dialog.Conversations(a.defineConversations)).
//		Kinds(a.defineCells, a.defineKinds).
//		Controls(a.bindKeys).
//		Spawn(a.spawn).
//		Scenes(a.defineScenes).
//		Update(a.update)
//
// # The world and the plugins
//
// [Start.World] makes the Stage's world from its world.Config each time the Stage is entered;
// [AfterWorld.Plugins] hands it to a function of the game's that makes the plugins over it, and
// the Stage uses them in the order made. [Start.Plugins] does the same for a Stage without a
// world, its plugins adding themselves to a plugin.Made.
//
// # The order
//
// Every section hands back a type that has the sections after it and none before, so a chain out
// of order does not compile; a section a game has no use for is left out, and Update alone must
// come. A section is given a function of whatever shape it needs ([Step]): with the
// game.Initializer or without, returning an error or not. Kinds and Spawn take several, run in
// their order. The order is that of what builds on what: the players before the kinds their units
// are of, the effects before the rules that cast them, the roles before the commands for their
// players and the kinds that play them, everything before the keys, and the world before the
// scenes that show it.
//
// # The plugins' sections
//
// A plugin may have a part of the definition of its own, between two of the Stage's
// (plugin/section.Define). Any link from World to Controls takes With(sections...), a plugin's
// Section run where it is written with the plugin it is of; one written out of its place panics
// as the chain is built.
//
// # What the Stage does itself
//
// The Stage so defined seeds a fresh game with Spawn's steps; entered from a save it spawns
// nothing. Once the world is there it makes the stack of the scenes Scenes hands back, shows the
// first (or those [AfterScenes.Shows] names; a loaded game those shown when it was saved) and
// tracks the stack's Composition for the saves. The scenes are made before the plugins populate
// the world: nothing in them reads the entities spawned.
//
// # A thing in its section
//
// A section defines its things under names and hands nothing on: an effect, a role, a command, a
// kind is registered in the Stage's world (Define) and taken back by its name (Named) in whichever
// later section builds on it. As a section begins the Stage tells the engine (package
// plugin/section), and the plugins refuse what is defined in another: an effect outside Effects, a
// role outside Rules, a kind outside Kinds, keys bound outside Players and Controls, a board or
// units seeded outside Spawn. A Stage written by hand is in no section, and nothing is refused.
package stage
