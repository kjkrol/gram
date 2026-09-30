// Package topography is a map in relief over a board: the ground's heights, the light and the
// water on them, drawn on the GPU, and the views of it — from above, isometric and in perspective.
//
// [NewPlugin] takes the world, the board and the [Config] — the views' sizes and reach, whether a
// fresh game begins isometric, how the ground is shaped (relief.Shaping) and what its slopes cost
// (relief.Climbing) — and puts the board in relief at once: it is the board's Map
// (board.Plugin.WithMap) — its Look, its Dressing, its heights and its costs — the world's Ground
// and Look, and the maker of the world's cameras. The world must have heights
// (world.Config.Heights) and may not wrap.
//
// # Packages
//
// The plugin composes its subpackages, none of which imports it; it registers their systems,
// gathers their commands' queues ([Plugin.Queues]) and keys ([Plugin.DefaultBindings]) and hands
// them its sky:
//
//   - relief: the heights, what climbing them costs, their shaping (Raise, Lower, Level), their
//     entity saved with the game and every mover's altitude;
//   - painter: the board's Dressing in relief and the board painted flat for the GPU — the Styles
//     of the kinds, their blends and coasts, the ways and the bridges;
//   - water: the sea's and the running water's materials and how the water is painted;
//   - terrain: the ground over a square grid, a mesh of the relief's lattice;
//   - hexes: the ground over a hex grid, a prism a cell;
//   - billboards: the world's entities standing on the relief, and their shadows;
//   - cameras: the views, their cameras and their control.
//
// # Relief
//
// The ground's heights are the plugin's relief.Relief ([Plugin.Relief]); [Plugin.Seed] raises it
// to a function when a game starts fresh (relief.MeanOfCells builds one from a height per cell).
// Every step the topography puts every unit at the ground under it plus its Mover's Lift, so a unit
// never declares where it stands in height and a hawk declares only how high it flies. A kind's
// Height is what stands on the cell: [Plugin.Top] is the cell's top as drawn, the ground and the
// kind together, which the cameras pick on. The slope multiplies the kind's cost, in the planner
// ([Plugin.Climb], [Plugin.Least]) and on the move ([Plugin.Slope], through the board's Moving
// behavior), off a road and on one, unless its kind is Graded (board.CellKind.Graded): a road cut
// into the slope costs its own price alone. The relief is the board's Heights ([Plugin.Heights]),
// which sight and navigation read through the board.
//
// # Styles
//
// How a kind looks in relief beyond its sprite is its painter.Style, set by its name
// ([Plugin.Style]).
//
// # Light and shadows
//
// The relief stands under an [Atmosphere] — plugins/atmosphere's Plugin ([Plugin.WithAtmosphere]),
// or, given none, sky.DefaultSun in still, clear air: its sun lights and shades it, its weather
// leans what sways, lays the clouds' shadows and hazes the far off. The ground is lit on the GPU,
// every pixel by the sun on the slope under it; the relief and what stands on it cast shadows,
// baked on the GPU as the sun moves — a strip a frame while it goes on, all at once when it leaps
// ([Plugin.WithShadows] turns them off; [CoarseShadows], H, or [Plugin.WithCoarseShadows] bakes
// them half as fine a side, softer, for about half the GPU's work). The world's entities are lit
// by the sun on level ground, lean with the wind and cast their shadows on the relief away from the
// sun, from above as in relief (plugins/topography/billboards).
//
// # The ground on the GPU
//
// [Plugin.Renderer] draws the ground, a render.Direct at the Ground tier for the scene's composer
// beside the board's and the world's, and the board's tiles lay nothing (board.Nothing): over a
// square grid the terrain's mesh, over a hex grid the hexes' prisms.
//
// # Commands
//
// The plugin is a plugin.CommandHandler: the cameras' commands (cameras.View, Turn, Tilt, LookOut,
// Look, Follow, Drive, LookFrom, LookAt), carrying the camera of whoever gave them, given the
// selection by [Plugin.WithSelection]; the relief's (relief.Raise and relief.Lower, = and - under
// the cursor, relief.Level, a left drag with L held — at once, in the tactical pause too); and
// [CoarseShadows] (H), which switches the shadows' detail. Call [Plugin.RunPlan] after the world
// has moved and before the players' RunPlan.
package topography
