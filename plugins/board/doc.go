// Package board lays a square or hex grid over the game world, with per-cell terrain (who may
// pass, what a step costs) and occupancy. Entities on the board move at the terrain's cost, solid
// cells push them out and veiled ones dim sight, and plugins/navigation builds pathfinding on top.
// A board stands only in a world with heights (world.Config.Heights).
//
// # Packages
//
// What a game uses is this package and its public subpackages: cell (one cell: its id and kind,
// its ground and way, what stands on it, the moment cell.Now, its occupancy), unit (an entity on
// the board: unit.At, unit.Mover, unit.Standing), grid (the topology), look (how the board is
// drawn), ground (what its ground is to the other plugins) and network, water (ways across the
// board). The machinery is in plugins/board/internal.
//
// # Board, Layout and kinds
//
// [Plugin], built over a grid.Grid, a cell.Occupancy and the world plugin, seeds its terrain from
// a [Layout] when the Stage starts fresh. Every cell is an entity carrying cell.Plot, cell.Ground,
// cell.Way and cell.Built, saved with the game, so an effect on it is an effect on the terrain.
// Kinds are defined through [Plugin.CellKinds]; [Board] is the one place to read and write the
// terrain.
//
// # Rules
//
// Every step the board runs the rules of its moments, which the roles obey: a unit.Standing for
// every entity on the board (the cell under it, its kind on the unit's level, whether it has
// Fallen where its domain may not be) and a cell.Now for every cell. Both are host.Placed: a
// rule's Around(n) acts on the cells within n rings of those under the entity and the entities
// standing on them (0: those under it alone). A Standing is host.Headed too: Ahead(n) acts on
// the cells one to n steps in front of the unit, the way it faces. The same pass writes every
// unit's steering.Pace from the ground under it.
//
// # Entities standing on the board
//
// A lever, a trapdoor, a gate, a plate is an entity standing still on a cell: [NewEntities] binds
// a game's rows of them and [Entities.Define] makes a kind, carrying cell.On and whatever the
// game gives besides, a cell.Floor, a cell.Wall, a cell.Rail, the roles it plays. The board
// gathers them on the cells' entities every step (cell.Built), and [Board.Kind] and [Board.Bare]
// are the cells with them. A Floor with a Height is a deck, and the cell has two levels
// ([Board.KindAt], [Board.KindFor], [Board.CellFor], [Board.LevelFor]); one with a Width and
// Links is a band ([Board.DecksOver], [Board.Reaches]), its railings never stepped across
// ([Board.Fenced]); a step along it takes a body no wider than its [Board.Lane]. A unit is no
// bigger than a cell's side ([NewUnits] panics past it), so any other step takes it.
//
// # Bridges
//
// A bridge, the cells whose Graded bands join along their links, runs straight and level from a
// head to a head, its deck [Bridges].Clearance over the ground between ([Plugin.WithBridges]).
// A level laying one that breaks the rules fails with its file and line; one broken later is
// not built, said in the log. Where one may be built is plugins/bridges'.
//
// # Level
//
// A game's map is a YAML file; kinds stay in the game's code, the file names them.
//
//	board:
//	  default: grass
//	  legend:
//	    ".": grass
//	    "~": {kind: water, plays: [lake], group: lake}
//	  cells: |
//	    ..~~..
//	  entities:
//	    - {kind: trapdoor, group: west trapdoors, at: [2, 0]}
//	relief:
//	  cells: |
//	    0 0 2.5 2.5 0 0
//	ways:
//	  roads: {road: {width: 12, paths: [[[0, 0], [5, 0]]]}}
//	  rivers:
//	    - [{at: [1, 0], kind: brook, width: 3}, [1, 1]]
//	units:
//	  - {kind: scout, side: player, name: ann, start: [0, 1]}
//
// [Plugin.Lay] seeds the board out of the file in the Spawn section and hands back a [Level]:
// its relief for a topography's Seed ([Level.Heights]) and its entities, which [Spawn] seeds the
// world with a type of row at a time, the row's fields tagged yaml being the entry's keys:
//
//	level, err := s.board.Lay(mapFile, "map.yaml")
//	s.topography.Seed(level.Heights())
//	return board.Spawn[unitRow](level, board.Sides{PlayerSide: {players.Give{To: s.player.ID}}})
//
// A [Sides] is the commands the game gives its entities as they are made. An error says the file
// and the line; an entity no Spawn took fails the board's Populate. A road never runs over a
// river: a point {onto: [x, y]} leads it onto the bridge or the ford standing there. With a
// relief every cell of a Graded road lies level or on an even ramp (ground.RoadFits).
//
// # Commands
//
// [Grid] shows or hides the board's grid (B; [Plugin.ShowGrid] from a scene); [Holds] the cells
// the occupancy holds (Shift+O, [Plugin.ShowHolds], drawn by [Plugin.HoldsRenderer]). Hand the
// board to players.NewPlugin for the keys.
//
// # Ways
//
// A cell.Way is what runs across a cell over its ground, a band Width wide from the cell's
// middle out towards each neighbour its Links name. [Board.Way] and [Board.SetWay] read and write
// them, [Board.Along] tells a step along a way from one over the ground beside it, [Board.Bare]
// is that ground. [Board.CellVersion] and [Board.Changes] count the changes, so whoever keeps
// something worked out of a cell knows when it is stale; [Board.Touch] counts one made beyond
// the board.
//
// # Heights, ground and occupancy
//
// The board is flat: what is high or steep is its [Map]'s (plugins/topography), which prices
// every slope ([Map.Climb], [Map.Least], [Map.Slope]). The board is the ground the other plugins
// meet: [Plugin.Heights], [Plugin.Surfaces], [Plugin.Cover] for sight, and its Solid cells the
// solid ground collision pushes colliders out of ([Plugin.WithCollision]). Its cell.Occupancy
// tracks who holds each cell; the board has every unit hold the cell it stands on and lets go of
// the holds of whoever left the world.
//
// # Drawing
//
// The board's own Map is the simple map: a flat ground seen from above, every kind in its sprite,
// the ways as bands. [Plugin.WithMap] puts another in. How a kind looks is its tile's, gathered
// on the board's [Atlas] as [Plugin.Renderer] is first asked for, every sprite the cell's size
// across (a game declaring looks by hand starts its own, [Plugin.NewAtlas]); a state of the
// ground is an effect on its cells, shown as a cover ([Plugin.Covering], [Slot.Under]).
//
// # Tiles and figures
//
// A [Tile] is a kind of cell in one piece: Define its cell.Kind and the roles its cells play,
// Look its look on the board's [Atlas] and its covers under effects, and, [Stateful] or
// [Ruled], its own effects and rules. A [Figure] is a kind of entity standing on the board the
// same way: Define over the board (NewUnits, NewEntities), Look on the world's atlas as any
// figure's. [Plugin.Tiles] and [Plugin.Figures], called in the Stage's Figures link, hand them
// on to the world's figures, which the Stage leads through its sections; [Atlas.Looks]
// declares the tiles' looks, the world atlas's the figures'.
//
// # Vocabulary
//
// Moments, for a role's rules: unit.Standing (every unit a step; host.Placed and host.Headed; its
// words unit.Fallen, unit.StandingOn, unit.OnKind) and cell.Now (every cell a step; host.Placed).
// Knobs, for an effect's Alter: cell.Ground (a cell's kind), cell.Floor, cell.Wall, cell.Rail
// (what stands on it). Commands, a player's: [Grid], [Holds] (looks).
package board
