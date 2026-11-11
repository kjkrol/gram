// Package underground digs what lies under the ground by one set of rules, package site's,
// whoever digs it: a level's file, a player with a tunnel in hand, a map maker.
//
// [NewPlugin] takes a world with heights, the board and the [Config]: where a structure may lie
// (site.Rules) and the kind of tunnel a player digs. A game defines its kinds in the Kinds
// section ([Plugin.Types]): a tunnel ([Types.Define] a [Spec]: whom its floor carries and how
// much it bears, how wide it runs and how much room lies over its floor) and a chamber
// ([Types.Chamber]).
//
// # A structure is its parts
//
// A structure is entities lying under the board's cells, saved as any: a piece under every cell
// it runs under, its floor a cell.Floor sunk under the ground (a negative Height) with the room
// over it (Clear), on a storey of its own, so the ground over it stays ground and a bridge may
// stand over it; a tunnel's pieces are bands running from one to the next, a chamber's whole
// cells. A tunnel comes out square to its head's edge and runs straight through its first
// cell, its mouth framed, the way in open. Over them stands the handle, "<name>.whole", which
// takes its pieces with it as it goes (world.Whole).
// A unit walks in at a mouth and through on the floor, no taller than the room, and a body in
// the room is kept in by its walls, as collision reads them of the board; a camera riding
// it stays under the roof; the ground over a mouth is cut away, and inside the structure is
// drawn with its own lamps, lit by none of the sun.
//
// # A level's structures
//
// A line of a level's file `{kind: tunnel, from: [x, y], to: [x, y], via: [[x, y]]}` is a tunnel
// from head to head turning at the cells via, `{kind: cave, from: [x, y], to: [x, y], depth: d}`
// a chamber under a rectangle of cells: [Write.Spawn] digs every one in the Spawn section and
// fails with the line of one that may not lie.
//
// # A tunnel in a player's hand
//
// [Plugin.DefaultBindings] have Shift+T take a tunnel in hand ([Tool]); while the player holds
// it, the cursor lays it out ([Aim]), a left click marks a head ([Mark]) and a right click or Esc
// puts it down. The tunnel in hand is drawn yellow where it may not lie ([Plugin.Reason] says
// why), green where it may. [Plugin.Window] asks "Dig the tunnel?": [Dig] has it dug at the next
// step of the simulation, its heads levelled on the ground [Plugin.WithShaper] gives, [Cancel]
// digs nothing. A tunnel confirmed waits for its step in Resources.Orders, saved with the game.
//
// # Seen from above
//
// From over the ground the courses of the tunnels and the units in them are marks over the world
// where the setting says ([Courses], under the settings' section Game; Settings.HideCourses).
//
// # Figures
//
// A [Figure] is a kind of structure in one piece: Define its Spec on the plugin's Types, Look
// the look of its mouths on the world's atlas, and, [Stateful] or [Ruled], its own effects and
// rules. [Plugin.Figures], called in the Stage's Figures link, hands them on to the world's
// figures; [Plugin.World] is the world they lie under.
//
// # Vocabulary
//
// Marks: world.Whole on a structure's handle, world.PieceOf on its pieces, world.Unseen on those
// under the ground. Commands, a player's with the tunnel in hand: [Aim], [Mark], [Dig],
// [Cancel]; a setting's: [Courses].
package underground
