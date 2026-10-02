// Package camera holds gram's top-down camera behind the camera.Camera contract, with its window
// arithmetic and wrapping. NewFromSpace and NewFromSpaceWithConfig build it; the world plugin does
// it for a game (world.Plugin.Camera, NewCamera) unless a view plugin gives the world its own.
package camera
