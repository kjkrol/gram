// Package landscape dresses a board's tiles beyond their sprites (a board.Dressing): the sun's
// light on the relief and the terrain's shadows, grounds blending into one another, coasts, water
// glinting and running, the ways across the cells, the clouds' shadows, and less of it all far
// off. A board without a landscape draws its sprites in even light; a game that wants no more
// never imports it, and the shader it brings never compiles.
//
// # Styles
//
// How a kind looks beyond its sprite is its [Style], set by its name ([Plugin.Style]): its Shine,
// its Flow, its Spread, whether it lies Under the others. The board's kinds keep what play needs;
// an effect that turns a kind into another (snow, ice) turns it into that kind's Style too, and a
// way's kind is styled the same way.
//
// # Light and shadows
//
// A tile is lit by the world's sun per corner, from the slope of the ground there and at the
// neighbours', so a slope runs on without a seam, and an upright face as much as the top's edge
// over it: a map in relief, from above as through any other look; a flat world keeps its sprites'
// colours on level ground and shows its slopes alone. The terrain casts shadows: a corner the
// ground or what stands on it hides from the sun, walked towards it up to 16 cells, gets the
// ambient light alone. Shadows and light are worked out as cells come into sight and kept until the
// terrain or the sun changes; [Plugin.WithShadows] turns the shadows off.
//
// # Water
//
// A kind with a Shine glints ([Glint], the SeaGlint material of water.kage): the shader ripples its
// surface with small waves and throws the sun back towards the eye, and within a few cells of a
// shore — the nearest cell that does not shine, worked out per corner of a square grid as the
// terrain changes ([Shore]) — the waves face it, roll in and break into foam. Water of a kind with a
// Flow runs instead ([Stream], the RunningWater material): down the slope of its cell, read off its
// corners, as fast as the Flow by the square root of the slope, averaged at each corner over the
// running cells meeting there, carried as a flow map — ripples and flecks of foam flowing on
// without a seam — and white where it runs fast: a rapid, a waterfall. The clouds of the world's
// weather shadow every tile once, over all that lies on it ([OvercastOn], the CloudShadow material
// of overcast.kage).
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
// A board.Way is drawn as a band through its cell: each way out ends halfway to its neighbour, as
// wide as the mean of the two ways there; the two out to the widest neighbours are one band curving
// round the cell's middle, any other joins it curving in, so a winding stream bends smoothly; a way
// out to one neighbour alone ends square across itself. A way whose kind shines is water running
// down its band; a way's Fade has it show the less the further it has faded, down to nothing where
// it ends, its water running on level ground the way it fades: a river running out into the sea.
// Ways lie on a tier just over the tiles; a band running slantwise reaches into the cells either
// side of the corner it runs through, so its last stretch takes the depth of the nearest of the
// four cells meeting there.
//
// # Kept, and less far off
//
// Nothing of this is worked out per frame: a cell's read is kept while board.Board.CellVersion
// says it is as it was, a tile's blends and way (placed as if it stood at 0, 0) while the cells
// round it are, its light while the terrain and the sun are. Far off, less is drawn: a tile's
// detail eases its water's glint and running out between 12 and 6 pixels a cell and its shore
// below 16, a way's between 24 and 16, and the shader leaves out waves finer than a pixel or two.
//
// # The ground sheet
//
// Where a cell spans fewer than 16 pixels on a square grid, a tile's blends and the ways over it
// are not drawn piece by piece: they are painted once on a ground sheet — the board's atlas with the
// board's cells below it, 16 pixels a cell, the tiles' sheet (board.Dressing's Sheet) — and the
// tile draws its top and then all that lies on it as one piece of the sheet, in its light. A cell is painted anew when it
// or a cell round it changes, the whole board at once when many do. So a far view hands the frame
// about as many pieces as the sprites alone.
package landscape
