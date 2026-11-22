// Package mapfile reads a map kept in files, in metres: a manifest, the ground's heights as an
// image and what lies on the ground as GeoJSON, whole or cut into tiles. It is the geography a
// board lays itself by, whichever its grid; the format is README.md's.
//
// # File
//
// [Load] reads a [File]: its [Heights], its [Feature]s with their places turned to the
// world's, y down, the parts of the manifest that are a reader's own ([File.Part]) and its
// lines tagged highway as [Road]s from [Meeting] to meeting ([File.Network]).
//
// # Tiles
//
// [Cut] writes a map as tiles of two tiers, the map as it is and its [Overview], which Load
// reads alike: heights as a place is asked for, features by [File.Within].
package mapfile
