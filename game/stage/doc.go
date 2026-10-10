// Package stage defines a game.Stage a section at a time, always in the same order: a chain
// begun with [New] and ended with Update, each link one part of the game and nothing else.
//
//	func newDemo() (*arena, game.Game) {
//		s := &arena{}
//		return s, game.New(props,
//			stage.New(MeadowStage).
//				World(worldCfg).                                   // the world…
//				Plugins(s.makePlugins).                            // …and the plugins the arena makes over it
//				Players(s.definePlayer).                           // the players, the plugins' default keys
//				Effects(s.defineEffects).                          // the states
//				Rules(s.defineRules).                              // the roles, the rules, the plans
//				Commands(s.defineCommands).                        // what can be asked for
//				With(dialog.Conversations(s.defineConversations)). // a plugin's own section
//				Kinds(s.defineCells, s.defineKinds).               // the kinds of cells and of units
//				Controls(s.bindKeys).                              // the game's own keys, each a command
//				Spawn(s.spawnCells, s.spawnUnits).                 // a fresh game: its board, its units
//				Scenes(s.defineScenes).                            // the scenes, once the world is there
//				Update(s.update))                                  // the tick
//	}
//
// # The world and the plugins
//
// [Start.World] makes the Stage's world from its world.Config each time the Stage is entered;
// [AfterWorld.Plugins] hands it to a function of the game's — the arena's makePlugins — that makes
// the plugins over it, keeping them, each telling the world of itself as it is made
// (world.NewSelf). The Stage uses them in the order made: the order a game defines its plugins in
// is the order they are used in, the game touching no game.Initializer. [Start.Plugins] does the
// same for a Stage without a world, its plugins adding themselves to a plugin.Made; a menu's has
// neither.
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
// # The plugins' sections
//
// A plugin may have a part of the definition of its own, between two of the Stage's
// (plugin/section.Define): the dialogs' Conversations, after the commands their answers give and
// before the kinds whose Scripts name their nodes. Any link from World to Controls takes
// With(sections...), a plugin's Section run where it is written, with the plugin it is of among
// those the Stage uses; one written after the part it comes before, or before the part it comes
// after, panics as the chain is built. What a plugin's section defines it refuses elsewhere.
//
// # What the Stage does itself
//
// The Stage so defined keeps its name and seeds a fresh game with Spawn's steps; entered from a
// save (game.Load, the engine's) it spawns nothing. Once the world is there — loaded, else spawned
// at the end of Spawn — it makes the stack of the scenes Scenes hands
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
// bound outside Players and Controls, a board, its heights or units seeded outside Spawn, a
// dialog's changes and files outside Conversations. How a thing is drawn is no section's: the
// looks are declared on the world's atlas in a scene's Layers (world.Plugin.NewAtlas). World and
// Plugins take anything, for the plugins define what is their own as they are made. A Stage
// written by hand — its own Init,
// Spawn and Update — is in no section, and nothing is refused.
package stage
