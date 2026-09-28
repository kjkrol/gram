// Package board lays a square or hex grid over the game world, with per-cell terrain
// (passability, movement cost, sprite) and occupancy tracking. Entities on the board
// move at the terrain's cost, solid cells push them out and veiled ones dim sight, and
// plugins/navigation builds pathfinding on top.
//
// # Board, Grid and Layout
//
// A [Grid] is a topology behind neighbor, coordinate and distance queries; [DefaultGrids] makes a
// square or a hex one, and each wraps per axis following the world's edges. A [Board] is a Grid
// with its terrain, the one place to read the topology and read or write terrain. [Plugin], built
// over a Grid, an [Occupancy] and the world plugin, seeds its terrain from a [Layout] (a default
// kind for every cell, per-cell overrides, and the heights) when the Stage starts fresh, and slows
// every entity carrying a [Mover] by the terrain under it (a Moving behavior it registers on the
// world).
//
// # Cell, CellKind and Terrain
//
// A [CellID] names one cell; [Cell] is an entity's current one. A [CellKind] is a named terrain:
// its movement cost, the [Domain]s it admits, whether it is solid (a wall) or how much it veils
// sight (a forest), and the sprite drawn for it; kinds are created
// through the Plugin's [CellKindDict]. Cost 1 is full speed and the cheapest step — a road; above
// 1 slows and costs more to plan through — the ground off a road, the islands' at 2.5.
// [CellKind.Costing] prices a kind differently for some domains — elves through a forest, a
// witch over snow — and [CellKind.CostFor] is what an entity pays: the cheapest of its domains
// the kind admits and prices, else Cost. A Graded kind — a road, a bridge, built up and cut into
// the slope — is not slowed by the slope under it, nor priced by it in a route: its Cost is the
// whole price. [Terrain] is what a cell answers about itself.
//
// # Ways
//
// A [Way] is what runs across a cell over its ground — a brook, a river, a road: a band Width wide
// from the cell's middle out towards each neighbour its [Links] name, a bit for each of the grid's
// directions ([Link] finds the bit for a neighbour, [Toward] the neighbour for a bit). Every cell
// entity carries one beside its Plot and Ground, the zero Way where nothing runs, so it is saved
// with the cell and an effect may alter it — a stream freezing over. Its kind decides who may cross
// the cell and what it costs there ([Way.Over]; [Board.Kind] is the ground as whoever crosses it
// meets it), the ground keeps the rest: whether it is solid, what it veils. [Board.Way] and
// [Board.SetWay] read and write it, [Layout.Ways] seeds it; [Board.Along] tells a step along a way
// — the way links the two cells — from one over the ground beside it, and [Board.Bare] is that
// ground, the cell's kind bare of the way, which is how a route follows a road round its bend
// instead of cutting the corner; the Map draws it as a band through the
// cell's middle — plain on the simple map, water and roads in relief on a topography's. [Board.CellVersion] counts the changes to each cell alone
// — its kind, its way, its heights, an effect on it — and [Board.Changes] to them all, so whoever
// keeps something worked out of a cell knows when it is stale.
//
// # Heights and slopes
//
// The board is flat. What is high, steep or in relief is its [Map]'s: plugins/topography keeps
// the ground's heights, shapes them, prices every slope and lights the tiles by them, and the
// board asks it — [Map.Top] for a cell's corners and level, [Map.Climb], [Map.Least] and
// [Map.Slope] for what a step and the speed cost beyond the kind's ([Plugin.Top], [Plugin.Climb],
// [Plugin.Least], [Plugin.Slope] delegate). A [CellKind]'s Height is what stands on the cell — a
// wall, a forest — in a world with heights (world.Config.Heights); a flat world refuses one, and its
// units carry no Z. Whoever changes a cell beyond the board — the topography shaping its corners
// — says so with [Board.Touch], so the cell's version moves and whatever was worked out of it is
// read anew.
//
// # Occupancy
//
// The board is the ground: [Heights] is the height of the ground at a point, the Map's — a
// topography's relief — and nil on a flat map ([Plugin.Heights]); [Cover] is what stands on the
// board and holds sight back, the Board itself ([Plugin.Cover]); and the Solid cells are the solid
// ground collision pushes colliders out of ([Plugin.WithCollision], collision.Field). Sight takes
// them with vision.Plugin.WithBoard. The world knows none of it: it knows its entities.
//
// [Occupancy] tracks who holds each cell and in which domains, gating and recording every step
// navigation takes when it keeps units a cell each (navigation.CellSpacing; units kept apart by
// their boxes leave it unasked): [SingleOccupancy] lets one entity per domain into a cell (a walker and a
// hawk share one, two walkers do not), [MultipleOccupancy] any number — tokens on a square, which
// carry no Physics, since bodies cannot overlap. A Solid cell stops only whoever its kind keeps
// out, so a wall admitting Air lets a flyer over.
//
// # Map and Renderer
//
// A [Map] is what the board is drawn and priced by beyond what its cells say: how the cells lie on
// the screen ([Look]), what lies over them beyond their sprites ([Dressing]), how high they stand
// and what a step costs. The board's own is the simple map: a flat world seen from above
// ([FlatLook]), every kind in its Color or drawn sprite, the ways and the crossings as plain bands
// in their kinds' colours, a step at its kind's cost times the distance. [Plugin.WithMap] puts
// another in — plugins/topography, a map in relief — and [Plugin.Map] is the one in use.
//
// How a kind looks is its [CellKind.Color], or a sprite drawn for it ([CellKindDict.Draw]), or
// whatever the game's own atlas has at its SpriteID: [Plugin.WithRenderer] takes the atlas, and
// given nil draws from the board's own ([Plugin.DefaultAtlas]), a cell's size each.
//
// [Plugin.WithRenderer] builds the [Renderer], a render.Source for a scene's render.Composer
// ([NewRenderer] for a board no plugin runs): it reads each visible cell and hands it, as a [Tile]
// — its box, its sprite, its kind — to the Map's Look, which lays it on the render.Ground tier:
// from above its sprite over its box, or as the Map has it. A kind with a Sway — trees, set by an
// effect when the wind blows — leans its top with the wind ([Tile.Sway]).
//
// Over a cell's ground may run a [Way] — a river, a road — and over that a [Crossing] — a bridge:
// the way decides who may cross the cell and at what cost, the crossing lets whoever it admits
// over too, the water running on under it ([Board.Kind]).
//
// The Map's Dressing lays what lies on the tiles beyond their sprites: the renderer hands it each
// frame first and takes from it the sheet the tiles are drawn from ([Tile].Atlas: the board's
// atlas or a sheet of the dressing's with the atlas on it), the tile asks it its [Tile.Base] and
// its [Tile.Light] and [Tile.FaceLight], and the Look has it lay what lies on the tile
// ([Tile.Dress]). The simple map's lays the bands; a topography's the sun's light on the relief
// and the terrain's shadows, grounds blending, coasts, water glinting and running, the ways drawn
// across the cells, the clouds' shadows. Without a Dressing's light a tile is drawn as it is; a sky
// over a flat board (atmosphere.Plugin.WithBoard) lights it by the hour.
// [RenderState] holds the renderer's live toggles, such as the grid: on a square grid each tile
// outlined by the shader along its own edges at no piece of its own (render.Frame.Tile) — where a
// Dressing lays grounds or ways over it ([Tile.Covered]), outlined by the dressing over them
// instead (render.Frame.OutlineOn); on a hex grid the cells' outlines as lines on a tier just
// above the tiles. It is left out where a cell spans fewer than a few pixels on screen.
//
// A Dressing that is [Parallel], under a Look that is a [ParallelLook], dresses the tiles on
// several goroutines at once ([Plugin.WithWorkers]; as many as there are CPUs unless told
// otherwise, none for a few tiles): the renderer Warms every visible tile on its own goroutine,
// has the dressing Ready itself, then shares the tiles out in runs, each drawn by a Worker of the
// dressing and of the look into a frame of its own, appended in order — the picture one goroutine
// would draw, piece for piece. The topography's dressing is one; the simple map's is not.
//
// The board is the [Cover] sight is held back by ([Board.Walk]) and reads the cells' cover all at
// once when asked ([Board.Ready], the [Readied] contract), for whoever walks it from several
// goroutines at a time.
package board
