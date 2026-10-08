// Package camera holds gram's top-down camera behind the camera.Camera contract, with its window
// arithmetic and wrapping. NewFromSpace and NewFromSpaceWithConfig build it; a game gets it from the
// cameras plugin (cameras.TopDown) unless a view plugin gives its own.
package camera
