// Package bridges builds bridges by one set of rules, package site's, whoever builds them: a
// level's file, a player with a bridge in hand, a map maker.
//
// [NewPlugin] takes the world, the board and the [Config]: where a bridge may stand (site.Rules)
// and the kind of bridge a player builds. A game defines its kinds of bridges in the Kinds
// section ([Plugin.Types], [Types.Define] a [Spec]): whom the deck carries, how wide and thick
// it is, its railings and its piers.
//
// # A bridge is its parts
//
// A bridge is entities standing on the board, saved as any: a piece of deck on every cell from
// head to head, a band with its railings, and over a map in relief a pier between every two
// spans. [Type.Deck] and [Type.Pier] are their looks' slots for a scene's atlas.
//
// # A level's bridges
//
// A line of a level's file `{kind: bridge, from: [x, y], to: [x, y]}` is a bridge from head to
// head: [Plugin.Spawn] builds every one in the Spawn section and fails with the line of one that
// may not stand.
//
// # A bridge in a player's hand
//
// [Plugin.DefaultBindings] have Shift+B take a bridge in hand ([Tool]); while the player holds
// it, the cursor lays the bridge out ([Aim]), a left click marks a head ([Mark]) and a right
// click or Esc puts the bridge down. [Plugin.Renderer] draws the bridge in hand, yellow where
// it may not stand ([Plugin.Reason] says why), green where it may. [Plugin.Window] asks "Build
// the bridge?": [Build] has it built at the next step of the simulation, its heads levelled on
// the ground [Plugin.WithShaper] gives, [Cancel] builds nothing.
//
// # Figures
//
// A [Figure] is a kind of bridge in one piece: Define its Spec on the plugin's Types, Look the
// looks of its deck and pier on the world's atlas, and, [Stateful] or [Ruled], its own effects
// and rules. [Plugin.Figures], called in the Stage's Figures link, hands them on to the world's
// figures, which the Stage leads through its sections; [Plugin.World] is the world they stand
// in.
//
// # Vocabulary
//
// Commands, a player's with the bridge tool in hand: [Aim], [Mark], [Build], [Cancel].
package bridges
