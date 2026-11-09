// Package lod is the one ladder of levels of detail the world is ticked and drawn by: a
// [Level] for each sector of the world, from the scale it is seen at and how far it lies.
//
// # Levels
//
// A world's [Ladder] says a sector's level from the shown pixels a world unit spans there and its
// distance from the observer (coarser than a level past its ring, Rings, so a camera
// seeing far at one scale does not hold the world at that scale's level; nothing past the
// Sight); the level ticked is the fullest any observer sees it at, and each camera draws at its
// own. A system says by its [Paces] ([Exact], [Steady], [Slowing]) how
// often it goes in a sector by how many levels coarser that is than the finest any observer
// sees, the finest going every step. The words of the levels are a contract every kind of thing
// keeps:
//
//	level and word       a figure                     a tile             the ground
//	L0 close             its L0 look, else as at L2   as at L2           as at L2
//	L1 close, no detail  its L1 look, else as at L2   as at L2           as at L2
//	L2 full              sprite, edged, upright       covers, ways, grid full
//	L3 no detail         flat sprite                  tile and ways      no shore, still water
//	L4 shape             blob; what stands: sheet     colour             coarse, no waves
//	L5 colour            blob, thinned; sheet in half colour             coarser, albedo quartered
//	L6 nothing           nothing                      nothing            nothing
//
// The levels are three scales of two rungs each, exact and coarse, and the omission: the close
// scale from [Close] (L0, L1), the standard from [Standard] (L2, L3), the strategic from
// [Strategic] (L4, L5), and [Omitted] (L6), neither ticked nor drawn. The close scale is a game's
// to switch on by its thresholds, Scales[L0] and Scales[L1], a zero leaving that level out, so
// the zero Ladder never reaches it; a system of its detail keeps to it by world.Plugin.Within at
// [Exact], ticking only where someone looks so close, and a figure declares its look there with
// world.Slot.At. A sector at the close scale is the finest seen, so the standard one beside it
// goes two levels coarser in every system's Paces.
//
// What stands on the board goes as a figure; clouds and shadows keep to no level yet, a later
// stage. An aggregate (world.Aggregate) is its parts finer than [Strategic] and itself alone from
// it, folding once its sector has lain there the ladder's Fold seconds, a field's
// (world.Field) each sector apart; [Seed] is the number a place of it draws from, the same at
// every unfold. A unit walks cells finer than [Strategic] and the abstract graph of the regions
// from it (navigation). A descent realises the coarse state in fine terms as the fine processes
// would have run it, an ascent sums the fine into the coarse, both from the same state and seed;
// what the coarse level does not model the descent does not invent.
//
// A world no camera observes is ticked at the ladder's Unobserved level ([Fallback]), L4 unless
// set. A camera keeps the ladder's Keep share of a level's entities, the same ones every frame,
// and the figures' colour goes into the ground in texels of its Texel.
package lod
