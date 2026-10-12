package plugin

// Restorer is a tracked value called back right after Persistence.Load decoded its state.
type Restorer interface {
	Restore()
}
