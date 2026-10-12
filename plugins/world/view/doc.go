// Package view is what one pair of eyes sees of a world.
//
// # View
//
// A [View] is a rectangle of the world and the entities the space finds in it, as an
// [EntitySet]. The world keeps one a camera (world.Plugin.ViewFor) and the [System] refreshes
// each once a tick after movement; a View over the whole world simply sees everything.
package view
