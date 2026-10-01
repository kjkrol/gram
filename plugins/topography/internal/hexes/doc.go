// Package hexes is the ground of a hex board in relief drawn on the GPU: every cell a prism
// standing to its top, a face down to each lower neighbour, coloured from the board's tiles
// composed once from above (render.Still) — anew when the board or the relief changes — lit by the
// sun, shaded by the clouds and hazed as the terrain is, writing the frame's depth. A [Ground]
// ([New]) is a render.Direct at the Ground tier in place of the tiles.
package hexes
