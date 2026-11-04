// Package lod is the one ladder of levels of detail the world is ticked and drawn by: a
// [Level] for each sector of the world, from the scale it is seen at and how far it lies.
//
// # Levels
//
// A world's [Ladder] says a sector's level from the shown pixels a world unit spans there and its
// distance from the observer; the level ticked is the fullest any observer sees it at, and each
// camera draws at its own. A system says by its [Paces] ([Exact], [Steady], [Slowing]) how
// often it goes in a sector by how many levels coarser that is than the finest any observer
// sees, the finest going every step. The words of the levels are a contract every kind of thing
// keeps:
//
//	level  word       a figure                      a tile              the ground
//	L0     full       sprite, edged, upright        covers, ways, grid  full
//	L1     no detail  flat sprite                   tile and ways       no shore nor running water
//	L2     shape      blob of its mean colour       colour              coarse lattice, no waves
//	L3     colour     colour in the ground's sheet  colour              sheet
//	L4     nothing    nothing                       nothing             nothing
//
// What stands on the board goes as a figure; clouds and shadows keep to no level yet, a later
// stage. L4 is neither ticked nor drawn. An aggregate (world.Aggregate) is its parts at L0 and
// L1 and itself alone from L2, folding once its sector has lain there the ladder's Fold seconds,
// a field's (world.Field) each sector apart; [Seed] is the number a place of it draws from, the
// same at every unfold. A unit walks cells below L2 and the abstract graph of the regions from
// L2 (navigation). A descent realises the coarse state in fine terms as the fine processes would
// have run it, an ascent sums the fine into the coarse, both from the same state and seed; what
// the coarse level does not model the descent does not invent.
//
// A world no camera observes is ticked at the ladder's Unobserved level ([Fallback]), L2 unless
// set. A camera keeps the ladder's Keep share of a level's entities, the same ones every frame,
// and the figures' colour goes into the ground in texels of its Texel.
package lod
