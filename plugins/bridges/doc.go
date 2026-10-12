// Package bridges builds bridges by one set of rules — package site's — whoever builds them: a
// level's file, a player with a bridge in hand, a map maker (cmd/gramap plans its bridges with
// site too).
//
// [NewPlugin] takes the world, the board and the [Config]: where a bridge may stand (site.Rules)
// and the kind of bridge a player builds. A game defines its kinds of bridges in the Kinds
// section ([Plugin.Types], [Types.Define] a [Spec]): whom the deck carries, how wide and thick it
// is, its railings and its piers.
//
// # A bridge is its parts
//
// A bridge is entities standing on the board, saved as any: a piece of deck on every cell from
// head to head — a band (cell.Floor, Graded, its Links: a head's one way into the bridge, a span's
// both ways along it) with its railings (cell.Rail) — and, in a world with heights, a pier between
// every two spans: a still collider from the ground up to the deck's underside, drawn standing.
// With heights the deck is as thick as the Spec says and its pieces and piers are world.Blocks,
// whose shadows a relief bakes with its own. The board carries whoever crosses it and keeps it
// straight, level and its heads flat while it stands (board.Bridges); [Type.Deck] and [Type.Pier]
// are their looks' slots for a scene's atlas.
//
// # A level's bridges
//
// A line of a level's file `{kind: bridge, from: [x, y], to: [x, y]}`, the kind one of the
// game's kinds of bridges, is a bridge from head to head: [Plugin.Spawn] builds every one in the
// Spawn section, planned by the rules with its heads flat at its deck already, and fails with the
// line of one that may not stand.
//
// # A bridge in a player's hand
//
// The plugin is a plugin.CommandHandler; [Plugin.DefaultBindings] have Shift+B take a bridge in
// hand ([Tool], control.Take) and, while the player holds it, in place of what those do with
// nothing in hand: the cursor lays the bridge out ([Aim]), a left click marks its first head
// ([Mark]) and then, where it may stand, its other head, asking whether to build it; a right
// click or Esc puts the bridge down (control.Drop). [Plugin.WithRenderer] draws the bridge in
// hand from head to head at its deck's height, yellow where it may not stand — [Plugin.Reason]
// says why — green where it may. [Plugin.Window] asks "Build the bridge?": [Build] has it built at
// the next step of the simulation — in the tactical pause it waits for the time to run — its
// heads levelled at its deck first on the ground [Plugin.WithShaper] gives (a topography's
// relief; never so a road breaks), [Cancel] builds nothing, the first head still marked.
package bridges
