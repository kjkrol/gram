// Package ground is what a board's ground is to the other plugins: the height of it at a point
// ([Heights], a topography's relief; none on a flat map) and what stands on it and holds sight
// back ([Cover], walked along a ray; [Readied] to read it all at once, for whoever walks it from
// several goroutines). Sight and navigation read them (board.Plugin.Heights, Cover); the board is
// collision's solid ground too (board.Plugin.WithCollision).
package ground
