// Package topography is a map in relief over a board: the ground's heights, the light and the
// water on them, drawn on the GPU, and the views of it from above, isometric and in perspective.
//
// [NewPlugin] takes the world, the board and the [Config] and puts the board in relief at once:
// the plugin is the board's Map and the world's Ground and Look. The world must have heights
// (world.Config.Heights) and may not wrap. What a game and the other plugins both name is in two
// small packages, relief (Climbing) and painter (Style); the rest is the plugin's own, in
// plugins/topography/internal.
//
// # Relief
//
// The ground's heights are [Plugin.Relief], seeded by [Plugin.Seed] when a game starts fresh
// (board.Level.Heights, or ground.MeanOfCells from a height per cell) and shaped at play by
// [Raise], [Lower] and [Level] as the Config's [Shaping] says. Every step the plugin puts every
// unit at the surface under it, the ground or a deck carrying its domain within its step, plus
// its Mover's Lift; a unit carrying a world.Drift is gravity's. Slopes multiply a kind's cost
// as the Config's relief.Climbing says, in the planner and on the move, unless the kind is
// Graded. [Plugin.Flatten] lays a cell flat as a ground.Shaper does.
//
// # Views
//
// [Plugin.Views] is a maker of cameras for the cameras plugin, each beginning as its [Start]
// says; every camera of a world in relief is one of them. [View] goes round the views, [Turn]
// and [Tilt] move the camera, [Ride] takes it behind and inside the entity it is fastened to,
// [LookFrom] and [LookAt] place the eye in perspective and [Look] looks round riding.
//
// # Light and shadows
//
// The relief stands under an [Atmosphere] ([Plugin.WithAtmosphere]; sky.DefaultSun in still,
// clear air given none): its sun lights the ground per pixel and casts the shadows of the relief
// and of what stands on it, baked as the sun moves ([Plugin.WithShadows], [CoarseShadows]); its
// weather leans what sways, lays the clouds' shadows and hazes the far off. How a kind looks
// beyond its sprite is its painter.Style ([Plugin.Style]).
//
// # Drawing and commands
//
// [Plugin.Renderer] draws the ground, a render.Direct in the opaque pass of the scene's
// composer. The plugin is a plugin.CommandHandler for the views', the shaping's and the shadows'
// commands ([Plugin.Queues], [Plugin.DefaultBindings]); call [Plugin.RunPlan] after the world
// has moved and before the players' RunPlan.
//
// # Vocabulary
//
// Commands, a player's through its camera: [View], [LookFrom], [LookAt], [Look], [Ride], [Turn],
// [Tilt] (the views); [Raise], [Lower], [Level] (the shaping); [CoarseShadows] (a look).
package topography
