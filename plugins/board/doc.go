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
//     it; the [Layout] it is seeded from, a level file laid ([Plugin.Lay]); its [Map]; [NewUnits]
//     for a game's kinds of units, [NewEntities] for those standing on the board.
//   - cell: what is said of one cell — its id and kind, the domains it admits, its ground, way and
//     crossing, what stands on it (On, Floor, Wall, Built), the moment of it (cell.Now), its
//     occupancy.
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
// the roles a cell plays and what it is called (cell.Entry.Roles,
// cell.Entry.Name and Group), and the ways. The grid wraps per axis following the world's
// edges. Once the ECS is set up every cell is an entity for good — cell.Plot, cell.Ground,
// cell.Way, cell.Built, its roles, an entity.Label where it is called something, and whatever the
// world's roster gives every cell (Roster().Cell: a game's own component, a Load reading the
// cell's cell.ID) — saved with the game, so an effect on it is an effect on the terrain.
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
// Every step the board runs the rules of its moments, which the roles obey: of a unit.Standing for
// every entity on the board — the cell under it, its kind on the unit's level (a deck's, or what
// lies under), its box and domain, whether it is in the air (unit.Airborne, which gravity puts on);
// Standing.Fallen where the domain may not be, a unit pushed into the sea, never one in the air —
// and of a
// cell.Now for every cell: its entity, which cell, its kind now. Both are plugin.Placed, data
// alone: the board tells a rule, in its Tick, which cells lie round (plugin.Tick.Around), and a
// rule's Here acts on the cells under the entity (a cell itself) and the entities standing on them,
// its Around on the rings of neighbours round them too — a witch's frost, fire spreading over the
// ground, a lever beside a unit pulled; the units there are no part of it. A unit's Standing tells
// the effects on the cell under it (Standing.States, unit.Over). A cell plays the roles of its kind
// (cell.Kinds.Define) and its Layout entry's own (cell.Entry.Plays), and obeys their rules
// (rule.Part.Obeys), and a command finds a cell by its name or its group (entity.Named,
// entity.Group). In the same pass the board writes every unit carrying a unit.Mover its
// steering.Pace — the cost and the slope of the ground under it — which the world's velocity pass
// goes by from the next step; WithLog has a line written for each unit fallen where its domain may
// not be. [Plugin.Surfaces] is where whoever moves over the board stands in a world with heights —
// the ground, or a deck within its step — and where a slab over its head begins: what gravity
// stands its bodies on (plugins/gravity).
//
// # Entities standing on the board
//
// A lever, a trapdoor, a gate, a plate is no terrain but an entity standing still on a cell:
// [NewEntities] binds a game's rows of them to the board and [Entities.Define] makes a kind, each
// standing in a box at its cell's middle, carrying cell.On and whatever the game gives besides — a
// cell.Floor (a trapdoor over a pit), a cell.Wall (a gate), the roles it plays (a lever), a
// collision.Collider without Physics (a plate, pressed in a collision.Meeting). The board gathers
// their Floors and Walls on the cells' entities every step (cell.Built), and [Board.Kind] and
// [Board.Bare] are the cells with them: a trapdoor shut carries a walker over the pit, opened it
// lets it fall (unit.Standing.Fallen); a gate shut is planned round and pushed out of, in a world
// with heights up to its Height. A Floor's Height is a deck over the cell's level, and the cell has
// two levels — the deck and what lies under ([Board.KindAt], [Board.KindFor]: the level of whoever
// has its feet where they are): a unit steps onto the deck within its step from any side, passes
// under it where it fits under its slab (Thickness), falls off its edge under gravity; a flat world
// refuses a Height. A Floor with a Width and Links is a band that wide from the cell's middle out
// to the neighbours its Links name — a bridge's deck, the ground beside it the cell's own, through
// a corner reaching over the two cells flanking it ([Board.DecksOver], [Board.KindFor] at a point,
// [Board.Reaches]); a cell.Rail stands railings along both its edges, which collision keeps a
// walker in by and the planner never steps across ([Board.Fenced]). A ford is a Floor at the
// ground's level at a Cost of its own; a bridge's Floor is Graded, and [Board.Bare] leaves it out
// as it leaves a way: a step cutting the corner beside a bridge crosses the water under it.
// [Board.Built] reads what is built. NewEntities gives each a still world.Velocity, or the one a
// kind gives — its Dir the way a ford runs, which its look Faces — and a world.Lie: a band's
// entity is headed along it and lies its length, Width wide, edged by its railings.
//
// # Bridges
//
// A bridge — the cells whose Graded bands join along their links — runs straight and level from a
// head to a head: a band from the middle of a flat tile at its deck into the bridge alone, no
// railings on it, so the road comes onto it from any side; its deck Bridges.Clearance over the
// ground under the middle of every cell between ([Plugin.WithBridges]). One that is not panics as
// a fresh board is first gathered, with the file and the line of its entity in a level; one broken
// later — an effect, a spawn — is not built, its decks and railings gone, said in the log: what a
// built bridge must be. Where one may be built, and building it, is plugins/bridges'. Whoever
// stands on a deck stands on its cell ([Board.CellFor], [Board.LevelFor] saying whether on the
// deck), a band reaching over the cell beside the corner it cuts too. A plugin building its own
// out of a level's lines takes them with [Each]; a ground.Shaper lays a cell flat at a height.
//
// # Level
//
// A game's map is a YAML file (map.yaml beside the demos' code): kinds stay in the game's code,
// the file names them.
//
//	board:
//	  default: grass                   # the kind of every cell the drawing leaves to it
//	  legend:                          # a character a kind, or a kind with roles, a name, a group
//	    ".": grass
//	    "_": pit
//	    "~": {kind: water, plays: [lake], group: lake}
//	  cells: |                         # a character a cell, row by row; spaces left out
//	    ..__..
//	    ~~..~~
//	  entities:                        # what stands on the terrain
//	    - {kind: trapdoor, group: west trapdoors, at: [2, 0]}
//	relief:                            # metres through the world's Scale, a number a cell
//	  cells: |                         # (corners: one a corner, on a square grid)
//	    0 0 2.5 2.5 0 0
//	    0 0 0 0 0 0
//	ways:                              # paths of points [x, y], a branch beginning on another
//	  roads: {road: {width: 12, paths: [[[0, 0], [5, 0]], [[3, 0], [3, 1]]]}}
//	  rivers:                          # each a path down to where its water goes, no part of it
//	    - [{at: [1, 0], kind: brook, width: 3}, [1, 1]]  # a point may turn its kind and width
//	units:
//	  - {kind: scout, side: player, name: ann, start: [0, 1]}
//
// On a hex grid a row of the drawing is an r and x is q: indented a character a row, it draws the
// board as it is. In the Spawn section [Plugin.Lay] seeds the board out of the file and hands back
// a [Level]: its relief for a topography's Seed ([Level.Heights]) and its entities, which
// [Spawn] seeds the world with a type of row at a time — every entity whose kind has rows of that
// type, its kind, side, name and group its own keys, every other one a field of the row tagged
// yaml, a cell.ID written [x, y] however deep it lies:
//
//	type unitRow struct {
//		Start cell.ID `yaml:"start"`
//	}
//
//	level, err := s.board.Lay(mapFile, "map.yaml")
//	…
//	s.topography.Seed(level.Heights())
//	return board.Spawn[unitRow](level, board.Sides{PlayerSide: {players.Give{To: s.player.ID}}})
//
// A side is the commands the game gives its entities as they are made ([Sides]). An error says
// the file and the line: a level the board cannot lay seeds nothing, and an entity no Spawn took
// fails the board's Populate.
//
// A road never runs over a river: it leads onto the bridge or the ford standing there, a point
// {onto: [x, y]} at either end of its path, linked towards that cell without lying on it. With a
// relief, every cell of a Graded road lies as Transport Tycoon lays it (ground.RoadFits): level,
// or an even ramp of ground.RoadSlope at most it runs straight up — a diagonal, a bend, a fork, an
// end level; on a hex grid only between cells of one level (ground.RoadJoins). A level breaking it
// is refused with the path's line.
// cmd/gramap makes such levels of the Earth.
//
// # Commands
//
// [Grid] shows the board's grid, or hides it — a look, not saved — on B by default; [Holds] the
// cells the occupancy holds, each outlined in its holder's colour, on the ground or a little
// further in on a deck, on Shift+O ([Plugin.ShowHolds], drawn by [Plugin.HoldsRenderer] put in
// the scene's composer). Hand the board to players.NewPlugin for the keys.
//
// # Ways
//
// A cell.Way is what runs across a cell over its ground — a brook, a river, a road: a band Width
// wide from the cell's middle out towards each neighbour its cell.Links name, a bit for each of the
// grid's directions (grid.Link finds the bit for a neighbour, Grid.Toward the neighbour for a bit);
// a bridge or a ford over it is an entity standing on the board. Every cell entity carries a way,
// the zero one where nothing runs, so an effect may alter it — a stream freezing over. The way
// decides who may cross the cell and what it costs there, what stands on it lets whoever it carries
// over too, the ground keeps the rest: whether it is solid, what it veils ([Board.Kind]).
// [Board.Way] and [Board.SetWay] read and write them, [Layout.Ways] seeds them; [Board.Along] tells
// a step along a way from one over the ground beside it, and [Board.Bare] is that ground, which is
// how a route follows a road round its bend instead of cutting the corner. [Board.CellVersion]
// counts the changes to each cell alone — its kind, its way, its heights, an effect on it — and
// [Board.Changes] to them all, so whoever keeps something worked out of a cell knows when it is
// stale.
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
// the solid ground collision pushes colliders out of — in a world with heights each standing from
// below up to its kind's Height over its level, a collision.Band the entity's own must meet, one of
// no Height at every height — the cells a kind does not take the ground it never pushes one over (a
// collision.Field, [Plugin.WithCollision]). The board's field, inside it, is both. Sight takes them
// with vision.Plugin.WithBoard. The world knows none of it: it knows its entities.
//
// A cell.Occupancy tracks who holds each cell, on which of its levels and in which domains, gating
// and recording every step navigation takes when it keeps units a cell each
// (navigation.CellSpacing; units kept apart by their boxes leave it unasked):
// cell.SingleOccupancy lets one entity per domain into a cell on each level (a walker and a hawk
// share one, two walkers do not, but for one on a bridge's deck and one on the ground under it),
// cell.MultipleOccupancy any number — tokens on a square, which carry no Physics, since bodies
// cannot overlap. A hold is a booking: the cell a unit stands on and the one it steps into. The
// board lets go of the holds of whoever left the world, every step (cell.Occupancy.Release), so
// one fallen in blocks no cell, and has every unit hold the cell it stands on: a unit holding it
// nothing — spawned in the game, pushed or thrown while nobody leads it — lets go of the cell it
// was unit.At and is At the cell it stands on, with unit.Entered; one in the middle of a step holds
// both its cells and is left to whoever leads it. Under a raised deck a unit holds its cell on the
// level its feet are on, as it climbs onto the deck or falls off it. A Solid cell stops only
// whoever its kind keeps out, so a wall admitting Air lets a flyer over.
//
// # Drawing
//
// The board's own Map is the simple map: a flat world seen from above (look.FlatLook), every kind
// in its sprite, the ways and the crossings as bands cut out of their kinds' sprites, a step at
// its kind's cost times the distance. [Plugin.WithMap] puts another in — a topography's, a map in
// relief — and [Plugin.Map] is the one in use. How a kind looks is the game's alone, in a scene:
// whatever its atlas ([Plugin.NewAtlas]) has at the kind's SpriteID. [Plugin.WithRenderer] builds
// the look.Renderer over that atlas, and [Plugin.WithWorkers] says how many goroutines may share a
// frame's tiles. How the renderer composes the tiles is plugins/board/look's.
//
// # Covers
//
// A state of the ground is an effect on its cells, the cell staying the kind it is: the effect
// turns the kind's knobs (cell.Ground) and what lies on the cell is a cover — [Plugin.Covering]
// gives the slot of the board's atlas laid over the cells under the effect, along the line those
// cells draw across the tiles, not along their edges (the simple map, a square grid). The board's
// atlas is written as the world's is ([Plugin.NewAtlas]): Add a kind's look by its name and
// chain [Slot.Under] with the effect for the cover's drawer.
// [Board.States] are the effects on a cell; unit.Standing.States and unit.Over tell a rule of the
// cell under a unit.
package board
