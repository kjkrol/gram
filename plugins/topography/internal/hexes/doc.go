// Package hexes is the ground of a hex board in relief drawn on the GPU: every cell six triangles
// from its middle at its level out to its corners at theirs, as the relief lies, walls only down
// from the board's edge, its kind's Height not raised, as the terrain draws a square board;
// coloured from the board's tiles and ways drawn from above, lit by the sun the way the ground
// faces at the corners and hazed as the terrain is. A [Ground] ([New]) is a render.Direct in the
// opaque pass in place of the tiles.
package hexes
