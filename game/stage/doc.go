// Package stage defines a game.Stage a section at a time, always in the same order: a chain
// begun with [New] and ended with Update, each link one part of the game and nothing else.
//
//	func newStage() game.Stage {
//		s := &myStage{}
//		return stage.New(MeadowStage).
//			Plugins(s.usePlugins).               // ctx.UseWorld, ctx.Use
//			Players(s.definePlayer).             // the players, the plugins' default keys
//			Effects(s.defineEffects).            // the states
//			Rules(s.defineRules).                // the roles, the rules, the plans
//			Commands(s.defineCommands).          // what can be asked for
//			Kinds(s.defineCells, s.defineKinds). // the kinds of cells and of units
//			Controls(s.bindKeys).                // the game's own keys, each a command
//			Restore(s.restore).                  // a saved game, resumed…
//			Spawn(s.spawnCells, s.spawnUnits).   // …or a fresh one: its board, its units
//			Scenes(s.defineScenes).              // the scenes, once the world is there
//			Update(s.update)                     // the tick
//	}
//
// # The order
//
// Every section hands back a type that has the sections after it and none before, so a chain out
// of order does not compile; a section a game has no use for is left out, and Update alone must
// come, for it is what hands back the Stage. A section is given a function of whatever shape it
// needs ([Step]): with the game.Initializer or without, returning an error or not. Kinds and
// Spawn take several, run in their order.
//
// The order is that of what builds on what: the players before the kinds their units are of, the
// effects before the rules that cast them, the roles before the commands for their players and
// the kinds — of cells and of units — that play them, everything before the keys that ask for it,
// and the world before the scenes that show it.
// An effect altering a cell's Ground to another kind resolves that kind as it runs
// (cell.Kinds.Named in the Alter), the kinds being defined after the effects. How things look once
// drawn — the atlases, an effect's looks, a board's covers — is a scene's, in its Layers.
//
// # What the Stage does itself
//
// The Stage so defined keeps its name; it starts fresh unless [AfterControls.Restore] says how it
// resumes, and seeds a fresh game with Spawn's steps. Once the world is there — loaded at the end
// of Restore, else spawned at the end of Spawn — it makes the stack of the scenes Scenes hands
// back, shows the first — or those [AfterScenes.Shows] names; a loaded game those shown when it
// was saved — and tracks the stack's Composition for the saves. The scenes are made before the
// plugins populate the world: nothing in them reads the entities spawned. The engine hands the
// rules of the roles played to the plugins catching their moments.
//
// # By name
//
// A section defines its things under names and hands nothing on: an effect, a role, a plan, a
// command, a kind is registered in the Stage's world (Define) and taken back by its name (Named)
// in whichever later section builds on it. A game keeps the names as constants, so a name
// mistyped does not compile, and its Stage's struct holds its plugins and players alone.
//
// # A thing in its section
//
// As a section begins the Stage tells the engine (package plugin/section), and the plugins refuse
// what is defined in another: a plugin used outside Plugins, an effect outside Effects, a role or
// a plan outside Rules, a command outside Commands, a kind of cell or of unit outside Kinds, keys
// bound outside Players and Controls, a board, its heights or units seeded outside Spawn. How a
// thing is drawn is no section's: the looks are declared on the world's atlas in a scene's Layers
// (world.Plugin.NewAtlas). Plugins takes anything, for the plugins define what is their own as
// they are made. A Stage written by hand — its own Init,
// Spawn and Update — is in no section, and nothing is refused.
package stage
