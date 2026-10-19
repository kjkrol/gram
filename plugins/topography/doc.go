// Package topography is a map in relief over a board: the ground's heights, the light and the
// water on them, drawn on the GPU.
//
// [NewPlugin] takes the world, the board and the [Config] and puts the board in relief at once:
// the plugin is the board's Map, the world's Ground and Look and the cameras' Cage. The world must
// have heights (world.Config.Heights) and may not wrap. What a game and the other plugins both
// name is in two small packages, relief (Climbing) and painter (Style); the rest is the plugin's
// own, in plugins/topography/internal.
//
// # Relief
//
// The ground's heights are [Plugin.Relief], seeded by [Plugin.Seed] when a game starts fresh
// (board.Level.Heights, or ground.MeanOfCells from a height per cell) and shaped at play by
// [Raise], [Lower] and [Level] as the Config's [Shaping] says. Every step the plugin puts every
// unit at the surface under it, the ground or a deck carrying its domain within its step, plus
// its Mover's Lift; a unit carrying a motion.Drift is gravity's. Slopes multiply a kind's cost
// as the Config's relief.Climbing says, in the planner and on the move, unless the kind is
// Graded. [Plugin.Flatten] lays a cell flat as a ground.Shaper does.
//
// # Cage
//
// The plugin is a cameras.Cage for a game to hand its cameras (cameras.Config.Cage): a point is
// [Plugin.Free] over the ground, over the kind standing on its cell to its Height and out of a
// deck's slab; [Plugin.Span] is the lowest and the highest the ground reaches, [Plugin.Grain]
// how finely a line of sight steps near a deck.
//
// # Light and shadows
//
// The relief is lit by the light plugin's sun and lamps per pixel (light.White given none)
// and shaded as the light bakes it: the relief's shadows, which fall on the relief from then on
// (light.Plugin.OverRelief), and the Baked blocks' (render.GroundShade); the live shadows the
// light lays over the ground's depth. What stands receives the shadows at its height as its
// light.Shadow says: the relief's and the clouds' wherever it is, the blocks' near the ground.
// A cell takes what its kind's Takes says. The relief stands under an [Atmosphere]
// ([Plugin.WithAtmosphere]; still, clear air given none): its weather leans what sways and lays
// the clouds' shadows; its air hazes the ground and what stands on it only when asked
// ([Plugin.WithHaze], [Haze]): far off from an eye, over the ground from above, all clear by
// default. How a kind looks beyond its sprite is its painter.Style ([Plugin.Style]).
//
// # Drawing and commands
//
// The plugin draws the ground, a render.Direct in the opaque pass of the world's picture, handing
// the frame its heights (render.GroundHeights), the board's grid on it in the board's line
// (board.Surface), and under no atmosphere hands the frame its still sky, giving way to a sky
// some other source hands it. In a world cut into sectors the
// ground drawn is that of the sectors a camera sees, a coarser mesh (or, on a hex board, the
// prisms' tops) where it sees one coarsely, turning into the fog before its sight's end, and
// shaded with less care ring by ring out from the sector under the middle of the screen
// ([Config.Care], [Plugin.SetCare], [Care]: the far water a level mirror of the sky, the
// farthest ground without the baked shade and the lamps); asked to ([Plugin.ShowCare],
// [ShowCare]), it lays the care's bands over the ground, the boundaries between sectors of
// different care. [Plugin.Reporter] is the panel's line on it.
// [Plugin.Miniature] is the board painted flat as one picture, a minimap's ground. It is a
// plugin.CommandHandler
// for the shaping's, the haze's and the care's commands ([Plugin.Queues],
// [Plugin.DefaultBindings]); call [Plugin.RunPlan] after the world has moved and before the
// cameras' RunPlan.
//
// # Vocabulary
//
// Commands, a player's: [Raise], [Lower], [Level] (the shaping); [Haze], [Care], [ShowCare] (a
// look). Settings ([Settings], saved with the game, in [Resources]): the haze, the care, the
// care shown.
package topography
