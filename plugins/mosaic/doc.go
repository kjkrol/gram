// Package mosaic brings a ground laid as a mosaic of cells round points
// (github.com/kjkrol/mosaic): hills as fine as their shape asks, roads whose lines the cells'
// edges lie on, drawn sector by sector as finely as the world's ladder sees each. A game gives
// it a [Map] as the Stage is made, and its colours as a [Look] in its scene.
//
// # Map
//
// A [Map] is the ground's heights and what is laid on it: [Road]s of one [Section] and the
// [Junction]s their ends meet at. [Load] reads one from a level kept in files, as board/mapfile's
// README.md says: heights as an image, lines tagged highway as the roads, meeting
// where they share a point. [Cut] writes such a level as tiles, which Load reads alike.
//
// # Config
//
// [Config] says how far the ground drawn may lie from the map's at each level of the ladder,
// and how much of it is worked out anew a frame.
//
// # Vocabulary
//
// The plugin gives no moments, facts, knobs or commands.
package mosaic
