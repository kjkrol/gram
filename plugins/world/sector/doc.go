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
// beyond [Off]. A system leaves out what it does not run on by excluding the marks in its query,
// or runs on one band alone by including its mark. A band's Pace is kept so low that the fastest
// entity, taking a step Pace times as long, does not leap past half a cell.
//
// # Anchors
//
// [Anchors] are whoever knows where the players look: the cameras plugin hands the world the
// point each of its cameras observes (camera.Focused), a minimap's none.
//
// # Vocabulary
//
// Marks: [Band2], [Band3], [Band4], [Band5], [Off].
package sector
