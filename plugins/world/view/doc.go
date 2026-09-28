// Package view is what one pair of eyes sees of a world: a [View] is a rectangle of the world and
// the entities the space finds in it, as an [EntitySet] — a set of the world's entities by index.
// The world keeps any number of Views (world.Plugin.NewView over a source of bounds,
// world.Plugin.DropView) and its view [System] refreshes each once a tick, right after movement
// has rebuilt the space; a View whose bounds cover the whole world is not queried and simply sees
// everything, as does the zero View a Stage has before its first tick. The camera's View is what
// the entity renderer draws; a View over another camera or a remote player's bounds is the same
// thing.
package view
