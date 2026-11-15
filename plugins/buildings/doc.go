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
// floors: n}` is a building ([Row]); [Write.Spawn] takes every one in the Spawn section and
// [Write.Stand] stands one from the game's code. cmd/gramap writes them of OpenStreetMap's
// buildings.
//
// # Drawn at every level, an entity where its sector unfolds
//
// The plugin draws the buildings itself, from the file's lines, in the frame's light, the
// relief's shade and the air's haze ([Plugin.WithAir]): whole where the camera sees their sector
// at the close and standard scales, their bodies alone, without windows, under a level lid at
// the strategic one. No sheet holds them.
//
// A building is an entity only where its settlement's sector has come near: the game's
// settlement, a field aggregate, gives itself [Raise] under its world.Unfold, and the plugin
// spawns the buildings standing in that stretch as its parts, each of its kind, standing for
// good (world.Fixed), turned (world.Stand), casting its shadow on the ground (light.Baked) and
// playing [BuildingRole], by which it goes as the sector folds. What a unit may not walk through
// is the board's: a map gives the cells under a building a kind no walker is allowed on.
//
// # Vocabulary
//
//   - [Raise]: the command a settlement gives itself under a world.Unfold to have the buildings
//     of the stretch unfolding spawned as its parts.
package buildings
