// Package board lays a square or hex grid over the game world, with per-cell terrain
// (passability, movement cost, sprite) and occupancy tracking. Entities on the board
// move at the terrain's cost, impassable terrain can be made solid, and plugins/navigation builds
// pathfinding on top.
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
// # Terrain bodies
//
// Built [Plugin.WithCollision], the board makes its Solid terrain physical: every run of solid
// cells becomes an immovable entity in the world — tagged [Plugin.Body] in board's tag [Family], with a collider and an
// infinite mass, no sprite, no kind — so no unit ends a tick inside a wall and walls cut sight;
// a run of veiled cells becomes a body without a collider carrying a vision.Transparency of
// 1 - Veil, so a cone fades through it, on the world.Layers of the kind's Veils, so an observer
// whose Sight.Blockers miss them looks over it.
// A body is made of the boxes the grid gives for each cell ([Grid.CellBoxes]: one for a square,
// [HexCapStrips] strips over each cap of a hex, covering it from outside), merged along both axes
// up to [MaxBodyCells] a side, one kind at one altitude each. The bodies follow [Board.Version];
// call [Plugin.RunPlan] after collision's.
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
// flies. Units get their Z from the Shape, terrain bodies from their cells. A hill is heights on
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
// more than MaxStep. The board is a plugin.Commander in a Quasi3D world: = and - under the
// cursor, a left drag with L held. [Board.Lift] and [Board.Flatten] do the same from a game's code.
//
// # Occupancy
//
// [Occupancy] tracks who holds each cell and in which domains, gating and recording every step
// navigation takes: [SingleOccupancy] lets one entity per domain into a cell (a walker and a
// hawk share one, two walkers do not), [MultipleOccupancy] any number — tokens on a square, which
// carry no Physics, since bodies cannot overlap. Solid terrain bodies are on the world.Layers of
// whoever their kind keeps out, so a wall admitting Air lets a flyer over and cuts none of its
// sight.
//
// # Renderer
//
// [Plugin.WithRenderer] builds the [Renderer] drawing each cell's sprite from an atlas; put it
// under the entity layer, or into a render.Sorted with the world's renderer. There it submits each
// cell's top at its altitude plus its kind's Height and, through an isometric camera, the two faces
// towards the viewer wherever the ground drops to a neighbour (a cliff, down to sea level 0 off the
// board) or the kind stands tall (a wall), shaded as if lit from the upper left. [RenderState] holds
// its live toggles, such as grid lines (drawn only in the plain layer).
package board
