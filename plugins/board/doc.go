// Package board lays a square or hex grid over the game world, with per-cell terrain (who may
// pass, how fast one goes) and occupancy. Entities on the board move at the pace the terrain and
// the game's mobility give them, solid cells push them out and veiled ones dim sight, and
// plugins/navigation builds pathfinding on top. A board stands only in a world with heights
// (world.Config.Heights).
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
// Kinds are defined through [Read.CellKinds].
//
// # Read and Write
//
// The board's state is read through [Plugin.Read] and changed through [Plugin.Write]: Read(w) is
// worker w's of the game's pool (0 the game's own goroutine), each with queries and scratch of its
// own, so the workers read the board at once; Write is the game's goroutine's. Read(w).Board()
// reads the grid and the terrain ([BoardRead]), Write().Board() writes the terrain
// ([BoardWrite]). [NewBoard] is a board with no plugin, for a test.
//
// # Rules
//
// Every step the board runs the rules of its moments, which the roles obey: a unit.Standing for
// every entity on the board (the cell under it, its kind on the unit's level, whether it has
// Fallen where its domain may not be) and a cell.Now for every cell. The board names its places
// for cond.At and act.At: [Around](n) are the cells within n rings of those under the entity
// and the entities standing on them (0: those under it alone), [Ahead](n) the cells one to n
// steps in front of a unit, the way it faces (a cell faces nowhere). The same pass writes every
// unit's steering.Pace from the ground under it.
//
// # Entities standing on the board
//
// A lever, a trapdoor, a gate, a plate is an entity standing still on a cell: [NewEntities] binds a
// game's rows of them and [Entities.Define] makes a kind, carrying cell.On and whatever the game
// gives besides, a cell.Floor, a cell.Wall, a cell.Rail, the roles it plays. The board gathers them
// on the cells' entities every step (cell.Built), and [BoardRead.Kind] and [BoardRead.Bare] are the
// cells with them. What each kind carries beyond what it is given is its [StandingTemplate]'s
// (world.Plugin.Roster): standing still, its shadow Baked into the ground's shade (light.Shadow); a
// unit's is a live patch. Every cell carries what its [CellTemplate] asks besides the board's own.
// A Floor with a Height is a deck, and the cell has two levels ([BoardRead.KindAt],
// [BoardRead.KindFor], [BoardRead.CellFor], [BoardRead.LevelFor]); one with a Width and Links is a
// band ([BoardRead.DecksOver], [BoardRead.Reaches]), its railings never stepped across
// ([BoardRead.Fenced]); a step along it takes a body no wider than its [BoardRead.Lane].
// [BoardRead.Limits] is what a cell limits a body by beyond its kind, a floor's load and a wall's
// gap ([BoardRead.Limited]). A unit's box past a cell takes a footprint of cells a side
// (unit.Footprint), up to [DefaultFootprint] unless [Plugin.WithFootprint] sets another
// ([NewUnits] panics past it): an odd one laid round the cell its unit is At, an even one from
// it right and down, its unit standing on the corner its middle cells share (unit.Place).
// [BoardRead.Railed] says whether a railing runs along a cell's own band.
// [Plugin.UnitKinds] lists the kinds of unit defined over the board ([UnitKind]).
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
// [Grid] shows or hides the board's grid ([Write.ShowGrid] from a scene); [Holds] the cells the
// occupancy holds ([Write.ShowHolds], each an outline over its cell). Neither has a key: hand
// the board to players.NewPlugin for the settings screen.
//
// # Ways
//
// A cell.Way is what runs across a cell over its ground, a band Width wide from the cell's middle
// out towards each neighbour its Links name. [BoardRead.Way] and [BoardWrite.SetWay] read and write
// them, [BoardRead.Along] tells a step along a way from one over the ground beside it,
// [BoardRead.Bare] is that ground. [BoardRead.CellVersion] and [BoardRead.Changes] count the
// changes, so whoever keeps something worked out of a cell knows when it is stale;
// [BoardWrite.Touch] counts one made beyond the board. [BoardRead.Passage] and
// [BoardRead.CellPassage] count only the changes to what a step over a cell reads, so a route
// worked out stands while a forest sways or a cell is painted anew.
//
// # Heights, ground and occupancy
//
// The board is flat: what is high or steep is its [Map]'s (plugins/topography), which says how
// steep every step is ([Map.Rise]); how fast a unit goes over it is the game's cell.Mobility,
// given with [Plugin.WithMotion] and read by the pace and the planner alike ([Read.Pace]), so
// what the planner refuses the unit never walks. The board is the ground the other plugins
// meet: [Read.Heights], [Read.Footing], [Read.Cover] for sight, and its Solid cells the
// solid ground collision pushes colliders out of ([Plugin.WithCollision]). Its cell.Occupancy
// tracks who holds each cell; the board has every unit hold the cell it stands on and lets go of
// the holds of whoever left the world.
//
// # Drawing
//
// The board's own Map is the simple map: a flat ground seen from above, every kind in its sprite,
// the ways as bands, laid on the GPU every frame and lit per pixel under the light plugin by its
// distant light, the shadows it baked and the lamps, as each cell takes them (cell.Kind.Takes).
// [Plugin.WithMap] puts another in. How a kind looks is its tile's, gathered on the board's
// [Atlas] once the Stage is set up, every sprite the cell's size across (a game
// declaring looks by hand closes its own before, [Plugin.NewAtlas]); a state of the
// ground is an effect on its cells, shown as a cover ([Plugin.Covering], [Slot.Under]). In a world
// cut into sectors the cells drawn are those of the sectors a camera sees: plainly, undressed and
// with no grid where it sees them coarsely, fading into the fog before its sight's end.
// [Write.Miniature] is the board flat from above as one picture, a minimap's ground.
//
// # Tiles and figures
//
// A [Tile] is a kind of cell in one piece: Define its cell.Kind and the roles its cells play,
// Look its look on the board's [Atlas] and its covers under effects, and, [Stateful] or
// [Ruled], its own effects and rules. A [Figure] is a kind of entity standing on the board the
// same way: Define over the board (NewUnits, NewEntities), Look on the world's atlas as any
// figure's. [Plugin.Tiles] and [Plugin.Figures], called in the Stage's Figures link, hand them
// on to the world's figures, which the Stage leads through its sections; [Atlas.Looks]
// declares the tiles' looks, then the surfaces', the world atlas's the figures'.
//
// # Surface
//
// A [Surface] is how the board looks beyond its tiles: its Look sets on the board's [Atlas] the
// [Line] the grid is drawn in ([Atlas.Grid]) and how wide a cell held is outlined
// ([Atlas.Holds]). [Plugin.Surfaces], called in the Stage's Figures link, notes them, a later
// setting taking the place of an earlier; none draws [DefaultGridLine] and [DefaultHoldWidth].
//
// # Vocabulary
//
// Moments, for a role's rules: unit.Standing (every unit a step; places [Around], [Ahead]; its
// words unit.Fallen, unit.StandingOn, unit.OnKind) and cell.Now (every cell a step; places
// [Around]).
// Knobs, for an effect's Alter: cell.Ground (a cell's kind), cell.Floor, cell.Wall, cell.Rail
// (what stands on it). Commands, a player's: [Grid], [Holds] (looks).
package board
