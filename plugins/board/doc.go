// Package board lays a square or hex grid over the game world, with per-cell terrain (who may
// pass, what a step costs, how it looks) and occupancy. Entities on the board move at the
// terrain's cost, solid cells push them out and veiled ones dim sight, and plugins/navigation
// builds pathfinding on top.
//
// # Packages
//
// What a game uses is this package and its public subpackages; the board's own machinery — the
// cells' state and entities, the systems running the rules, the ground's field, the simple map's
// bands — is in plugins/board/internal, which nothing outside the board imports.
//
//   - board: the [Plugin]; the [Board], a grid and its terrain, the one place to read and write
//     it; the [Layout] it is seeded from; its [Map]; [NewUnits] for a game's kinds of units.
//   - cell: what is said of one cell — its id and kind, the domains it admits, its ground, way and
//     crossing, the game's tags of places, the moment of it (cell.Now), its occupancy.
//   - unit: an entity on the board — the cell it is At, how it moves (Mover), where it stands at a
//     step (Standing).
//   - grid: the topology — a Grid, DefaultGrids, the Link from a cell to its neighbour, a Shape.
//   - look: how the board is drawn — a Look, a Dressing, a Tile, the Renderer.
//   - ground: what its ground is to the other plugins — Heights, Cover, Readied.
//   - network, water: ways across the board; the rivers of a relief.
//
// # Board, Layout and kinds
//
// [Plugin], built over a grid.Grid, a cell.Occupancy and the world plugin, seeds its terrain from
// a [Layout] when the Stage starts fresh: a default kind for every cell, per-cell overrides with
// the game's tags of places, the roles a cell plays and the wire it is wired to (cell.Entry.Roles,
// cell.Entry.Wired), the ways and the crossings. The grid wraps per axis following the world's
// edges. Once the ECS is set up every cell is an entity for good — cell.Plot, cell.Ground,
// cell.Way, cell.Crossing, its tags and roles, a rule.Wired where it is wired, and whatever the
// world's roster gives every cell (Roster().Cell: a game's own component, a Load reading the
// cell's cell.ID) — saved with the game, so an effect on it is an effect on the terrain. A cell
// wired to a wire no world defined (world.Plugin.Wire) panics as the cells are made.
//
// A kind is a named terrain: its movement cost, whom it admits, whether it is solid (a wall) or
// how much it veils sight (a forest), and the sprite drawn for it; kinds are created through
// [Plugin.CellKinds]. Cost 1 is full speed and the cheapest step — a road; above 1 slows and costs
// more to plan through — the ground off a road, the islands' at 2.5. cell.Kind.Costing prices a
// kind differently for some domains — elves through a forest, a witch over snow — and
// cell.Kind.CostFor is what an entity pays: the cheapest of its domains the kind admits and
// prices, else Cost. A Graded kind — a road, a bridge, built up and cut into the slope — is not
// slowed by the slope under it, nor priced by it in a route: its Cost is the whole price.
// cell.Terrain is what a cell answers about itself; the Board is one.
//
// # Rules
//
// Every step the board runs the rules hooked on it ([Plugin.Hook], or game.Initializer.Hook, which
// finds the board for them): of a unit.Standing for every entity on the board — the cell under it,
// its kind and the game's tags of its place, its box and domain; Standing.Fallen where the domain
// may not be, a unit pushed into the sea — and of a cell.Now for every cell: its entity, which
// cell, its kind now, and whether the centre of an entity carrying unit.At lies on it this step
// (Now.Trodden; Now.Stood for a rule's If: a plate pressed). Both are plugin.Placed, data alone:
// the board tells a rule, in its Tick, which cells lie round (plugin.Tick.Around), and a rule's
// Here acts on the cells under the entity (a cell itself), its Around on the rings of neighbours
// round them too — a witch's frost, fire spreading over the ground. Rules of a cell.Now filter
// cells by the game's tags of places (rule.Self: a zone), and a Standing tells those of the cell
// under a unit (Standing.Places). A cell playing a role obeys the role's rules (rule.Part.Obeys: a
// trapdoor, a plate), and one wired to a wire follows it: OnWire drives the wire (a plate putting
// it on while stood on), WhileWire reads it (the trapdoors on it open while it is on). In the same
// pass the board writes every unit carrying a unit.Mover its steering.Pace — the cost and the
// slope of the ground under it — which the world's velocity pass goes by from the next step;
// WithLog has a line written for each unit fallen where its domain may not be.
//
// # Ways
//
// A cell.Way is what runs across a cell over its ground — a brook, a river, a road: a band Width
// wide from the cell's middle out towards each neighbour its cell.Links name, a bit for each of
// the grid's directions (grid.Link finds the bit for a neighbour, Grid.Toward the neighbour for a
// bit); a cell.Crossing — a bridge — runs over it. Every cell entity carries both, the zero ones
// where nothing runs, so an effect may alter them — a stream freezing over. The way decides who
// may cross the cell and what it costs there, the crossing lets whoever it admits over too, the
// ground keeps the rest: whether it is solid, what it veils ([Board.Kind]). [Board.Way],
// [Board.SetWay], [Board.Crossing] and [Board.SetCrossing] read and write them, [Layout.Ways] and
// [Layout.Crossings] seed them; [Board.Along] tells a step along a way from one over the ground
// beside it, and [Board.Bare] is that ground, which is how a route follows a road round its bend
// instead of cutting the corner. [Board.CellVersion] counts the changes to each cell alone — its
// kind, its way, its heights, an effect on it — and [Board.Changes] to them all, so whoever keeps
// something worked out of a cell knows when it is stale.
//
// # Heights and slopes
//
// The board is flat. What is high, steep or in relief is its [Map]'s: plugins/topography keeps the
// ground's heights, shapes them, prices every slope and lights the tiles by them, and the board
// asks it — Map.Top for a cell's corners and level, [Map.Climb], [Map.Least] and [Map.Slope] for
// what a step and the speed cost beyond the kind's ([Plugin.Climb] and [Plugin.Least] delegate).
// A cell.Kind's Height is what stands on the cell — a wall, a forest — in a world with heights
// (world.Config.Heights); a flat world refuses one, and its units carry no Z. Whoever changes a
// cell beyond the board — the topography shaping its corners — says so with [Board.Touch], so the
// cell's version moves and whatever was worked out of it is read anew.
//
// # Ground and occupancy
//
// The board is the ground: ground.Heights is the height of the ground at a point, the Map's — a
// topography's relief — and nil on a flat map ([Plugin.Heights]); ground.Cover is what stands on
// the board and holds sight back ([Plugin.Cover], a ground.Readied too); and the Solid cells are
// the solid ground collision pushes colliders out of — in a world with heights each standing
// from below up to its kind's Height over its level, a collision.Band the entity's own must meet,
// one of no Height at every height — the cells a kind does not take the ground it never pushes
// one over (a collision.Field, [Plugin.WithCollision]). The board's field, inside it, is both. Sight takes them with vision.Plugin.WithBoard. The world knows none of it: it
// knows its entities.
//
// A cell.Occupancy tracks who holds each cell and in which domains, gating and recording every
// step navigation takes when it keeps units a cell each (navigation.CellSpacing; units kept apart
// by their boxes leave it unasked): cell.SingleOccupancy lets one entity per domain into a cell (a
// walker and a hawk share one, two walkers do not), cell.MultipleOccupancy any number — tokens on
// a square, which carry no Physics, since bodies cannot overlap. A hold is a booking: the cell a
// unit stands on and the one it steps into. The board lets go of the holds of whoever left the
// world, every step (cell.Occupancy.Release), so one fallen in blocks no cell. A Solid cell stops
// only whoever its kind keeps out, so a wall admitting Air lets a flyer over.
//
// # Drawing
//
// The board's own Map is the simple map: a flat world seen from above (look.FlatLook), every kind
// in its Color or drawn sprite, the ways and the crossings as plain bands in their kinds' colours,
// a step at its kind's cost times the distance. [Plugin.WithMap] puts another in — a topography's,
// a map in relief — and [Plugin.Map] is the one in use. How a kind looks is its Color, or a sprite
// drawn for it (cell.Kinds.Draw), or whatever the game's own atlas has at its SpriteID:
// [Plugin.WithRenderer] builds the look.Renderer over that atlas — given nil, the board's own atlas
// of the kinds, a cell's size each — and [Plugin.WithWorkers] says how many goroutines may share a
// frame's tiles. How the renderer composes the tiles is plugins/board/look's.
package board
