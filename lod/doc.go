// Package lod is the one ladder of levels of detail the world is ticked and drawn by: a
// [Level] for each sector of the world, from the scale it is seen at and how far it lies.
//
// # Levels
//
// A world's [Ladder] says a sector's level from the shown pixels a world unit spans there and its
// distance from the observer; the level ticked is the fullest any observer sees it at, and each
// camera draws at its own. A system says how often it goes at each level by its [Paces]
// ([Exact], [Steady], [Slowing]). The words of the levels are a contract every kind of thing
// keeps:
//
//	level  word       a figure                 a tile               the ground
//	L0     full       sprite, edged, upright   covers, ways, grid   full
//	L1     no detail  flat sprite              tile and ways        no shore nor running water
//	L2     shape      blob of its mean colour  colour               coarse lattice, no waves
//	L3     colour     nothing                  colour               sheet
//	L4     nothing    nothing                  nothing              nothing
//
// What stands on the board goes as a figure; clouds and shadows keep to no level yet, a later
// stage. L4 is neither ticked nor drawn.
package lod
