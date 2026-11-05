// Package view is what one pair of eyes sees of a world.
//
// # View
//
// A [View] is a rectangle of the world and the entities the space finds in it, as an
// [EntitySet]. The world keeps one a camera (world.Write.NewView) and the [System] refreshes
// each once a tick after movement; a View over the whole world simply sees everything.
//
// # Sectors seen
//
// In a world cut into sectors a camera's View ([NewSeen]) is of the sectors its picture covers
// within its sight (the world's lod.Ladder.Sight times camera.SightOf), each at the lod.Level the
// camera sees it at, lod.Omitted off the picture or past the sight: two drawn side by side a level
// apart at most, none finer than it is ticked at. It holds the entities of the sectors seen;
// [View.Level] and [View.Fog] say how fully one at a point is drawn and how far into the fog.
package view
