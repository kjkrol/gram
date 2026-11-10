// Package forest plants forests over the board: fields of trees standing as a camera comes near
// and folding into the world's sheets as it goes.
//
// [NewPlugin] takes the world and the board, whose cells and heights a forest reads. It has no
// pass of its own: its trees come and go by the world's moments of an aggregate.
//
// # Kinds of trees
//
// A game defines its kinds of trees in the Kinds section ([Plugin.Types], [Types.Define] a
// [Spec]): how tall a tree stands and how wide its crown, how it looks, how it casts its shade,
// whether it stops what collides, and the game's roles it plays beside [TreeRole]. A tree is a
// part of its forest standing for good (world.Fixed), going as its sector folds. At L2 it stands
// in relief as a billboard of its Side, or as a body of it on the four sides and over the top of
// its box where its Spec says Body, at L3 as a billboard of its Top; from L4 out the world's
// sheets hold it as planted. A look left out is drawn of its [Colours]. Its looks are whole,
// drawn in render's cutout pass (world.Slot.Cutout), and canopies (world.Slot.Canopy): from
// above a unit standing under a tree shows through its crown where the world's Canopies setting
// says, in relief the crown hides it.
//
// # Stands
//
// A forest is a game's kind with world.Aggregate and world.Field playing a role the plugin makes
// in the Rules section ([Plugin.Stands], [Stands.Define] a [Stand]): each of its sectors come down
// to L3 plants trees over the kinds of cells the Stand names, as many a cell as its Density, each
// plot sown by the forest's seed (lod.Seed) and its tree drawn by weight among the [Share]s of the
// Mix growing at the height of the ground there. A forest plants the same trees each time, and
// the world previews them into its sheets before anyone comes near. A Stand's When has an effect
// of the forest's choose between two stands; [Stands.Felled] tells a forest folding short of its
// trees.
//
// # Figures
//
// A [Figure] is a kind of forest in one piece: Define its kinds of trees on the plugin's Types and
// its own kind, Look its own look on the world's atlas, and, [Stateful] or [Ruled], its effects
// and rules, its stands among them. [Plugin.Figures], called in the Stage's Figures link, hands
// them on to the world's figures; [Plugin.World] and [Plugin.Board] are what they stand on.
//
// # Vocabulary
//
// Roles: [TreeRole], every tree's. Words: [Stands.Felled], of a world.Fold.
package forest
