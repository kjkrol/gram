package plugin

// Publisher hands the renderer what it draws, once a frame after the frame's ticks: Publish reads
// the world into what is drawn next, and Swap, called while nothing draws, has it drawn. A
// tracked value, a source the Stage's plugins draw or a scene's drawer may be one; a scene's
// drawer publishes only while the scene is shown.
type Publisher interface {
	Publish()
	Swap()
}
