// Package sector is the world cut into sectors and the levels they are ticked at round the
// observers, the cameras the players look through.
//
// # Levels
//
// A world's Config.Sectors cuts it into square sectors, whole blocks of the space's buckets
// ([Grid]), and its Config.LOD says each one's level (lod.Ladder): every tick the world lays a
// sector at the fullest level any observer sees it at, and marks the entities in it: those at
// L0 carry no mark, those at L1 [L1] and so on, those at L4 [L4]. What lies at L4 is asleep, not
// in the world's space, so no query, sight or collision meets it until its sector comes back.
//
// An observer sees a sector finer at once and coarser only a tenth past a threshold and half a
// sector past its sight (lod.Ladder.Hold), and draws no two sectors side by side more than a
// level apart, lifting the coarser, though one at L4 may lie beside anything. A [Keen] entity
// keeps its sector and the eight round it ticked at its level at most, so those asleep there wake
// before it comes.
//
// # Paces
//
// A system goes at the levels as the plugin owning it says (lod.Paces); world.Plugin.Pace is the
// pace of its Paces, an option of the system's query. At L4 none goes.
//
// # Observers
//
// [Observers] are whoever knows where the players look: the cameras plugin hands the world each
// of its cameras, a minimap's none.
//
// # Vocabulary
//
// Marks: [L1], [L2], [L3], [L4]. Components: [Keen].
package sector
