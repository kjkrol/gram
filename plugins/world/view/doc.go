// Package view is what one pair of eyes sees of a world.
//
// # View
//
// A [View] is a rectangle of the world and the entities the space finds in it, as an
// [EntitySet]. The world keeps one a camera (world.Plugin.ViewFor) and the [System] refreshes
// each once a tick after movement; a View over the whole world simply sees everything.
//
// # Sectors seen
//
// In a world cut into sectors a camera's View ([NewSeen]) is of the sectors its picture covers
// within its sight (camera.Detail), each at a [Level]: Exact, Coarse where a world unit spans
// fewer screen units than the camera's Coarse, Fogged in the fog before the sight's end, Hidden
// past it, off the picture or beyond every band. It holds the entities of the sectors seen;
// [View.Level] and [View.Fog] say how finely one at a point is drawn and how far into the fog.
package view
