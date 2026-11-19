// Package buildings stands the buildings of a level's file as solids: walls on a rectangle turned
// any way and a roof over them, each as long, as wide and as high as its line says.
//
// [NewPlugin] takes a world with heights and the board. A game defines its kinds in the Kinds
// section ([Plugin.Types], in a [Figure]): a [Spec] says how high a storey is, how many
// a building has where the file says none, its colours, how wide a window's bay is and its
// [Part]s, the first its body on the building's own rectangle, the rest (a tower, a turret, a
// room on the roof) standing where the spec puts them, each under a [Roof]: [Flat], [Gable],
// [Hip] or [Pyramid]. Sizes are world units: a game gives them in metres through its
// world.Scale.
//
// # A level's buildings
//
// A line of a level's file `{kind: house, mid: [x, y], size: [length, width], turn: degrees,
// floors: n, door: [x, y]}` is a building ([Row]); [Write.Spawn] takes every one in the Spawn
// section and [Write.Stand] stands one from the game's code.
//
// # A door and a room
//
// A building whose line gives a door, of a kind whose Spec gives one its width and height, stands
// square to the board's square grid on whole cells, a room over them on [RoomStorey], entered by
// the door in the middle of the wall its line points at. The room's floor lies as high as the
// ground in the middle of the door, level across its cells: dug into the ground uphill, the
// ground cut where it rises into it, on a plinth downhill; the storeys are counted from it. Its
// handle is a room of the interiors plugin (interiors.Room) under a light.Shelter: the sun gets
// in by the windows of its storey where the ground outside lies under their sills and by its
// door, its lamp burning from dusk; [Plugin.WithRooms] takes the keeper of the rooms, so a room
// an uncovered entity stands in is drawn bared and its lamp as its shelter says.
//
// # Drawn at every level, an entity where its sector unfolds
//
// The plugin draws the buildings itself, from the file's lines, in the frame's light, the
// relief's shade and the air's haze ([Plugin.WithAir]): whole where the camera sees their sector
// at the close and standard scales, among the figures, giving way to what is uncovered as the
// world's settings say (world.Write.GivesWay), a room's door and the windows of its storey open
// and its inside drawn, lit by the sun where it falls in; their bodies alone, without windows,
// under a level lid with the ground at the strategic one. No sheet holds them.
//
// A building is an entity only where its settlement's sector has come near: the game's
// settlement, a field aggregate, gives itself [Raise] under its world.Unfold, and the plugin
// spawns the buildings standing in that stretch as its parts, each of its kind, standing for
// good (world.Fixed), turned (world.Stand), casting its shadow on the ground (light.Baked) and
// playing [BuildingRole], by which it goes as the sector folds. What a unit may not walk through
// is the board's: a map gives the cells under a building a kind no walker is allowed on, and a
// room's walls stand on its cells' edges but for its door.
//
// # Vocabulary
//
//   - [Raise]: the command a settlement gives itself under a world.Unfold to have the buildings
//     of the stretch unfolding spawned as its parts.
package buildings
