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
// through the Plugin's [CellKindDict]. Cost 1 is full speed and the baseline path weight; above 1
// slows and costs more to plan through; below 1 is a boost a game may choose to offer.
// [CellKind.Costing] prices a kind differently for some domains — elves through a forest, a
// witch over snow — and [CellKind.CostFor] is what an entity pays: the cheapest of its domains
// the kind admits and prices, else Cost. [Terrain] is what a cell answers about itself.
//
// # Domains, Mover and Standing
//
// A [Domain] is a way of moving — [Land], [Water], [Air], or a game's own bit — and a cell's
// Allows says which may stand on it: water admits Water, a hole nobody. An entity's [Mover] says
// which it uses (none means Land); the planner keeps it to cells that admit it. Every tick, after
// collisions, the board tells each entity carrying Cell where it stands as a [Standing] —
// [Plugin.RegisterBehavior] takes an [Each] of it, naturally one over Mover — and
// [Standing.Fell] says the entity is where its domain may not be: pushed into water, dropped
// into a hole. What follows is the
// game's: despawn, teleport, damage. Call [Plugin.RunPlan] every tick, after collision's.
//
// # Cell entities: Plot and Ground
//
// The terrain lives in the ECS. At Setup every cell becomes an entity for good — a [Plot] naming
// the cell and holding its [Relief], the heights of its corners, and a [Ground] holding its kind —
// or, after a load, the saved ones are found again; the board keeps only which entity is which cell's.
// Cell entities carry no world.Base: they are not in the world's space and do not count against
// its MaxCount. The Board reads and writes them ([Board.Kind], [Board.Set], [Board.Relief]), and
// until Setup, or on a board no ECS runs, it keeps a seed instead: a [TerrainMap] and the corner
// heights. [Plugin.CellEntity] is a cell's entity, so anything done to entities can be done to a
// cell: an effect altering Ground (or Plot) changes the terrain, and the board counts it in its
// [Board.Version] the tick it happens and the tick it ends, as effects.Active.Altered and
// effects.Idle say. A change never costs a pass over every cell: whoever writes says so. The kind
// and the heights are two components because an effect ending puts back the whole component it
// altered: a frost ending restores the kind and leaves the ground shaped meanwhile as it is.
//
// # Solid ground and cover
//
// Terrain is never an entity in the world's space: the [Board] is the world's solid ground and its
// cover (world.Field and world.Cover, set by [NewPlugin]), read straight from the cell entities
// whenever collision or sight asks, so a cell changed now counts from the next tick and costs
// nothing to change. [Board.Solid] gives collision the cells under a box that are Solid and keep
// out a layer the entity is on, each with the sides open where the neighbour is not, so a unit
// slides along a wall and never catches on the seam between two cells. [Board.Walk] gives sight
// the cells along a ray whose kind veils the observer's Sight.Blockers (the kind's Veils; zero
// veils everyone): τ is 1 - Veil, and in a Quasi3D world the cover stands from the cell's ground
// up by the kind's Height, so it casts a shadow and is looked over from above. Solid and Veil are
// apart: a fence is solid and hides nothing, a thicket hides and is walked through, a Warcraft
// forest is both. On a square grid both are exact, cell by cell; on a hex one Solid gives the
// boxes of [Grid.CellBoxes] (one for a square, [HexCapStrips] strips over each cap of a hex) and
// Walk steps a quarter of a cell along the ray.
//
// The board requires of every unit a [Cell] (where it starts) and a [Mover] (the domains it moves
// in) through the world's kind.Roster — and makes them itself in [Units]: a game binds its rows to
// the board once ([NewUnits]: the units' [Shape], where a row says a unit stands) and defines each
// kind by its Mover and steering profile plus its own components; Position and Cell come from the
// one point, Layers from the domain.
//
// # Heights
//
// In a Quasi3D world (world.Config.Quasi3D) the ground has heights, apart from the kinds: a cell's
// [Relief] holds its four corners, a [CellKind]'s Height is what stands on it. [Layout.Heights]
// raises the ground when the Stage starts fresh ([Board.SetHeights]: sampled at the corners, or at
// the centre of a hex, which is level; [MeanOfCells] builds one from a height per cell). The
// [Board] is the world's Ground ([Board.GroundAt], [Board.Step]): on a square grid it reads between
// a cell's corners, so a hill has slopes and a unit on a slope stands at its height. Every tick the
// board writes each Z-carrying entity's Altitude: the ground under its centre plus its Mover's
// Lift, so a unit never declares where it stands in height and a hawk declares only how high it
// flies. Units get their Z from the Shape. A hill is heights on
// cell entities and never a body, so the cost of sight does not depend on how many a game has.
// The renderer draws the tiles sloped, lit from the upper left so the relief reads, and faces only
// where a top stands above its neighbour's — a wall over grass, a raised edge over the sea. A flat
// world refuses heights, a Height or a Lift where it first meets one.
//
// # Shaping
//
// The ground changes as in Transport Tycoon: [Raise] and [Lower] move the corner nearest a point
// (a hex cell on a hex grid) by a [Shaping] Step, [Level] brings an area to the height where it
// began, and the ground round about follows until no two corners along a cell's edge differ by
// more than MaxStep. The board is a plugin.CommandHandler in a Quasi3D world: = and - under the
// cursor, a left drag with L held. [Board.Lift] and [Board.Flatten] do the same from a game's code.
//
// # Occupancy
//
// [Occupancy] tracks who holds each cell and in which domains, gating and recording every step
// navigation takes: [SingleOccupancy] lets one entity per domain into a cell (a walker and a
// hawk share one, two walkers do not), [MultipleOccupancy] any number — tokens on a square, which
// carry no Physics, since bodies cannot overlap. A Solid cell stops only whoever its kind keeps
// out, so a wall admitting Air lets a flyer over.
//
// # Renderer
//
// [Plugin.WithRenderer] builds the [Renderer], a render.Source for a scene's render.Composer: it
// reads each visible cell and hands it, as a [Tile] — its box, its sprite, the heights of its top
// and its neighbours' — to the board's [Look], which lays it on the render.Ground tier: from above
// its sprite over its box, unless a view plugin ([Plugin.SetLook], plugins/isometry) stands it up
// as a block with faces. In a world with heights the tile is lit by the world's sun ([Tile.Light]:
// per corner, from the slope of the ground there and at the neighbours', so a slope runs on
// without a seam; [Tile.FaceLight] for an upright face), from above as through any other look — a
// map in relief. The terrain casts shadows too: a corner the ground or what stands on it hides from
// the sun, walked towards it up to 16 cells, gets the ambient light alone, and a face as much sun as
// the top's edge over it. The shadows are worked out as cells come into sight and kept until the
// terrain or the sun changes; [Plugin.WithShadows] turns them off. A flat world is drawn as its
// sprites are.
// [RenderState] holds its live toggles, such as the grid: on a square grid each tile outlined by the
// shader along its own edges (render.Frame.Tile), costing no piece of its own; on a hex grid the
// cells' outlines as lines on a tier just above the tiles. It is left out where a cell spans fewer
// than a few pixels on screen.
package board
