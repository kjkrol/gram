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
// the kind admits and prices, else Cost. [Terrain] is what a cell answers about itself.
//
// # Slopes
//
// What is steep is the relief, not a kind. [Climbing] says what a slope does to whoever goes over
// it: a climb slows by Up per unit of rise over run; a descent is quickest, by Down, at a fall of
// Ease and slows past it by Steep a unit — a steep way down is picked carefully — and a Free domain
// (Air by default) flies over. The slope multiplies the kind's cost, on a road and off it. A cell's slope is read off its own corners, whichever way
// one goes across it: the board's Moving behavior applies it along the entity's heading, and
// [Board.Climb] prices a step by the slope of the cell it enters, which the planner multiplies
// into the kind's cost. [Plugin.WithClimbing] sets it, [DefaultClimbing] otherwise.
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
// [Board.SetWay] read and write it, [Layout.Ways] seeds it; a landscape (plugins/landscape) draws
// it as a band through the cell's middle. [Board.CellVersion] counts the changes to each cell alone
// — its kind, its way, its heights, an effect on it — and [Board.Changes] to them all, so whoever
// keeps something worked out of a cell knows when it is stale.
//
// # Heights
//
// The ground has heights, apart from the kinds: a cell's
// [Relief] holds its four corners, a [CellKind]'s Height is what stands on it. [Layout.Heights]
// raises the ground when the Stage starts fresh ([Board.SetHeights]: sampled at the corners, or at
// the centre of a hex, which is level; [MeanOfCells] builds one from a height per cell). In a
// Quasi3D world (world.Config.Quasi3D) the [Board] is the world's Ground ([Board.GroundAt], [Board.Step]): on a square grid it reads between
// a cell's corners, so a hill has slopes and a unit on a slope stands at its height. Every tick the
// board writes each Z-carrying entity's Altitude: the ground under its centre plus its Mover's
// Lift, so a unit never declares where it stands in height and a hawk declares only how high it
// flies. Units get their Z from the Shape. A hill is heights on
// cell entities and never a body, so the cost of sight does not depend on how many a game has.
// A view plugin draws the tiles sloped (plugins/isometry), and faces only where a top stands above
// its neighbour's — a wall over grass. The ground has no vertical walls:
// on a square grid neighbouring cells share the corners where they meet ([Board.SetRelief] moves
// the neighbours' with a cell's, and a relief an effect writes into one cell's Plot is sealed to
// its neighbours' the same tick). A flat world refuses a Height or a Lift where it first meets
// one, but its ground may have heights all the same: slopes cost ([Climbing]) and a landscape
// shades them, and no entity stands at a height.
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
// [Plugin.WithRenderer] builds the [Renderer], a render.Source for a scene's render.Composer
// ([NewRenderer] for a board no plugin runs): it reads each visible cell and hands it, as a [Tile]
// — its box, its sprite, the heights of its top and its neighbours' — to the board's [Look], which
// lays it on the render.Ground tier: from above its sprite over its box, unless a view plugin
// ([Plugin.SetLook], plugins/isometry) stands it up as a block with faces. A kind with a Sway —
// trees, set by an effect when the wind blows — leans its top with the wind ([Tile.Sway]).
//
// Over a cell's ground may run a [Way] — a river, a road — and over that a [Crossing] — a bridge:
// the way decides who may cross the cell and at what cost, the crossing lets whoever it admits
// over too, the water running on under it ([Board.Kind]).
//
// That is all a board draws alone: its sprites in even light. What a map needs beyond them — the
// sun's light on the relief and the terrain's shadows, grounds blending, coasts, water glinting and
// running, ways drawn across the cells, the clouds' shadows — is a [Dressing]'s, set by
// [Plugin.SetDressing] (plugins/landscape): the renderer hands it each frame first and takes from
// it the sheet the tiles are drawn from ([Tile].Atlas: the board's atlas or a sheet of the
// dressing's with the atlas on it), the tile asks it its [Tile.Base] and its [Tile.Light] and
// [Tile.FaceLight], and the Look has it lay what lies on the tile ([Tile.Dress]). [RenderState]
// holds its live toggles, such as the grid: on a square grid each tile outlined by the shader along
// its own edges at no piece of its own (render.Frame.Tile) — where a Dressing lays grounds or ways
// over it ([Tile.Covered]), outlined by the dressing over them instead (render.Frame.OutlineOn);
// on a hex grid the cells' outlines as lines on a tier just above the
// tiles. It is left out where a cell spans fewer than a few pixels on screen.
package board
