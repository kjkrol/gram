// Package painter is the topography's painter: the board's Dressing in relief and the board
// painted flat for the ground drawn on the GPU.
//
// A [Painter] ([New]) dresses the board's tiles as look.Dressing asks and paints them on the GPU
// into sheets ([Painter.Surface]): the board's albedo, its water in layers beside it and each
// pixel's kind; over a hex grid it hands the tiles drawn from above their still water glinting
// ([Painter.Overlay]) and draws the ways across them ([Painter.DrawWays]). [Painter.Wet] is where
// water may lie, [Painter.Shores] the way to the shore from every corner of a square grid.
//
// # Styles
//
// How a kind looks in relief beyond its sprite is its [Style], set by name ([Painter.Style]):
// its Shine, its Flow, its Spread, whether it lies Under the others, what it MixWith. An effect
// that turns a kind into another turns it into that kind's Style too; a way's kind is styled
// the same way.
//
// # Water
//
// A kind with a Shine glints (water.Glint): small waves, the sun thrown back towards the eye,
// and within a few cells of a shore a swell rolling in and breaking into foam. Water of a kind
// with a Flow runs instead (water.Stream), down the slope of its cell, white where it runs fast.
//
// # Blends and coasts
//
// Kinds with a Spread run into each other along a line their cells draw, not along the cells'
// edges: a neighbour's kind is weighed at each tile's corners, side middles and middle by the
// share of the cells meeting there and shown where the weight is over a half. A kind Under the
// others (water) is drawn as the tile's base, the land laid over it, so a coast runs round.
//
// # Ways
//
// A cell.Way is drawn as a band through its cell curving round the middle, as wide at each side
// as the mean of the two ways meeting there. A way whose kind shines is water running down its
// band; one whose Style names a kind to MixWith takes on that kind's look as far as its Mix; a
// way's Fade has it show the less the further it has faded. Water lies over a way as over the
// grounds round a coast, so a way shows only where the land does.
//
// # Details
//
// A kind with a Detail is textured where its cells are seen near: [Painter.Details] bakes every
// detail into one sheet, a tile each, its tone scaled to a mean grey and its hue toned down, and
// [Painter.Kinds] is the board painted as the albedo is but each pixel its kind's detail, edged
// hard where the albedo fades; the ground's shader blends the details between the pixels and
// multiplies the albedo by them, so the kind's colour stays and the detail gives the grain.
//
// # Painted once
//
// Nothing of this is worked out per frame: what the shaders read of a cell — its kind's sprite
// and Style, its way, the ground at its corners — is laid into an image, anew as the cell
// changes, and the GPU works out the shapes from it. Over a square grid the whole board is
// painted flat, the cells within two of a changed one anew, all of them when many changed, and
// the shores worked out anew as a cell comes to shine or stops; over a hex grid the tops are
// drawn anew, the ways with them, as the board changes or while water runs on a way.
package painter
