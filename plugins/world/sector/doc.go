// Package sector is the world cut into sectors and the bands of pace they lie in round the
// anchors, the points the players observe.
//
// # Bands
//
// A world's Config.Sectors cuts it into square sectors, whole blocks of the space's buckets, and
// lists its [Band]s: the first reaches nearest the anchors and goes at the world's pace, each one
// farther a step every Pace of the world's, and what lies beyond the last does not go at all.
// Every tick the world lays each sector in the band its nearest anchor reaches ([Grid.Place]),
// into a nearer band at once and into a farther one only past a margin, and marks the entities
// in it: those in the first band carry no mark, those in the second [Band2] and so on, those
// beyond [Off]. What lies beyond is not in the world's space, so no query, sight or collision
// meets it until its sector comes back into a band. A band's Pace is kept so low that the
// fastest entity, taking a step Pace times as long, does not leap past half a cell.
//
// # Policies
//
// A system goes in the bands as the plugin owning it says ([Policy]): [Everywhere] every step in
// every band, [Paced] a step every Pace in the farther ones over as many steps at once, [Near] in
// the first alone; beyond the last none goes. It asks a world.Pace of each block of its query.
//
// # Anchors
//
// [Anchors] are whoever knows where the players look: the cameras plugin hands the world the
// point each of its cameras observes (camera.Focused), a minimap's none.
//
// # Vocabulary
//
// Marks: [Band2], [Band3], [Band4], [Band5], [Off]. Policies: [Everywhere], [Paced], [Near].
package sector
