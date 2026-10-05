// Package stage defines a game.Stage a section at a time, always in the same order: a chain
// begun with [New] and ended with Update, each link one part of the game and nothing else.
//
//	func newStage() game.Stage {
//		s := &myStage{}
//		return stage.New("meadow").
//			Plugins(s.usePlugins).      // ctx.UseWorld, ctx.Use
//			Players(s.definePlayer).    // the players, the plugins' default keys
//			Cells(s.defineCells).       // the kinds of cells
//			Effects(s.defineEffects).   // the states
//			Rules(s.defineRules).       // the roles, the rules, the plans
//			Commands(s.defineCommands). // what can be asked for (ctx.Commands)
//			Kinds(s.defineKinds).       // the kinds of units
//			Controls(s.bindKeys).       // the game's own keys, each a command
//			Looks(s.defineLooks).       // the drawing rules
//			Scenes(s.defineScenes).     // the scenes
//			Layout(s.layOut).           // a fresh game's board
//			Units(s.placeUnits).        // a fresh game's units
//			Update(s.update)            // the tick
//	}
//
// # The order
//
// Every section hands back a type that has the sections after it and none before, so a chain out
// of order does not compile; a section a game has no use for is left out, and Update alone must
// come, for it is what hands back the Stage. A section is given a function of whatever shape it
// needs ([Step]): with the game.Initializer or without, returning an error or not.
//
// The order is that of what builds on what: the players before the kinds their units are of, the
// effects before the rules that cast them, the roles before the commands for their players and
// the kinds that play them, everything before the keys that ask for it. How things look once
// drawn — the atlases, an effect's looks, a board's covers — is a scene's, in its Layers.
//
// # What the Stage does itself
//
// The Stage so defined keeps its name, makes the stack of the scenes Scenes hands back, shows the
// first — or those [AfterScenes.Shows] names — and tracks the stack's Composition for the saves;
// it starts fresh unless [AfterShows.Restore] says how it resumes, and seeds a fresh game with
// Layout, then Units. The engine hands the rules of the roles played to the plugins catching their moments.
//
// # A thing in its section
//
// As a section begins the Stage tells the engine (package plugin/section), and the plugins refuse
// what is defined in another: a plugin used outside Plugins, a kind of cell outside Cells, an
// effect outside Effects, roles given to a plugin or a kind of cell outside Rules, commands handed over outside Commands, a
// kind of unit outside Kinds, keys bound outside Players and Controls, drawing rules outside
// Looks, a board seeded outside Layout, units outside Units. Plugins takes anything, for the
// plugins define what is their own as they are made. A Stage written by hand — its own Init,
// Spawn and Update — is in no section, and nothing is refused.
package stage
