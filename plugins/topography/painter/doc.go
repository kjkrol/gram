// Package painter is the topography's painter: the board's Dressing in relief — the light on the
// tiles and what lies on them, worked out of the board, its relief, the Styles of its kinds and
// the sky's sun and weather — and the board painted flat for the ground drawn on the GPU.
//
// A [Painter] ([New]) dresses the board's tiles as board.Dressing asks and paints them once into
// sheets ([Painter.Surface]): the board's albedo, 16 pixels a cell, and its water in layers
// (water.Layers) beside it; [Painter.Wet] is where water may lie, [Painter.Coast] the way to the
// shore from every corner of a square grid.
//
// # Styles
//
// How a kind looks in relief beyond its sprite is its [Style], set by its name ([Painter.Style],
// the topography's Plugin.Style): its Shine, its Flow, its Spread, whether it lies Under the
// others, what it MixWith. The board's kinds keep what play needs; an effect that turns a kind into
// another (snow, ice) turns it into that kind's Style too, and a way's kind is styled the same way.
//
// # Water
//
// A kind with a Shine glints (water.Glint, the SeaGlint material): the shader ripples its
// surface with small waves and throws the sun back towards the eye, and within a few cells of a
// shore — the nearest cell that does not shine, worked out per corner of a square grid as the
// terrain changes (water.Shores) — the waves face it, roll in and break into foam. Water of a kind with a
// Flow runs instead (water.Stream, the RunningWater material): down the slope of its cell, read off its
// corners, as fast as the Flow by the square root of the slope, averaged at each corner over the
// running cells meeting there, carried as a flow map — ripples and flecks of foam flowing on
// without a seam — and white where it runs fast: a rapid, a waterfall. The clouds' shadows lie over
// the water as over the ground.
//
// # Blends and coasts
//
// Kinds with a Spread run into each other along a line their cells draw, not along the cells'
// edges: over each quarter of a tile a neighbour's kind is weighed at the quarter's corners by the
// share of the cells meeting there that are of it and shown where the weight is over a half
// (render.Frame.SpriteBlend), fading in over the mean of the two Spreads; the weights at a corner
// or a side are the same from every tile, so a staircase of cells becomes a slant and a cell alone
// a rounded diamond. A kind Under the others — water — keeps its glint: a tile that spreads next to
// it is drawn as it and its own kind laid over it by the share of cells not under, and the water's
// tile has the land round it laid over it the same way, so a coast runs round.
//
// # Ways
//
// A cell.Way is drawn as a band through its cell: each way out ends halfway to its neighbour, as
// wide as the mean of the two ways there; the two out to the widest neighbours are one band curving
// round the cell's middle, any other joins it curving in, so a winding stream bends smoothly; a way
// out to one neighbour alone ends square across itself. A way whose kind shines is water running
// down its band; a way whose kind's Style names a kind to MixWith takes on that kind's look as far
// as its Mix, the other sprite glazed over its own and blended along the band (render.Frame.Glaze)
// — a river turning into the sea's colour towards its mouth; a way's Fade has it show the less the
// further it has faded, down to nothing where it ends, its water running on level ground the way
// it fades: a river running out into the sea. A cell.Crossing — a bridge — is cut into bands as a
// way is, on a tier over the ways, each band meeting whichever of its neighbour's way and crossing
// runs back to it: a road meets its bridge, not the river under it. A way running out into water no
// way runs across runs on to its middle under it: the water lies over a way as it lies over the
// grounds round a coast, so a way shows only where the land does and a river's end follows the
// coast. Ways lie on a tier just over the tiles; a band running slantwise reaches into the cells
// either side of the corner it runs through, so its last stretch takes the depth of the nearest of
// the four cells meeting there.
//
// # Painted once
//
// Nothing of this is worked out per frame: a cell's read is kept while board.Board.CellVersion
// says it is as it was, a tile's blends and way (placed as if it stood at 0, 0) while the cells
// round it are. The tiles are dressed only to be painted: over a square grid the whole board is
// painted flat — every cell's base, the grounds running in, the ways and the crossings, 16 pixels
// a cell, its water beside it — a cell anew when it or a cell round it changes, the whole board
// when many do; over a hex grid the tiles are composed once from above (render.Still), anew when
// the board or the relief changes. The shader leaves out waves finer than a pixel or two, so the
// water far off calms instead of flickering.
package painter
