package plugin

// Populator is a Plugin that seeds its own initial state, run only when a Stage starts afresh.
type Populator interface {
	Populate() error
}
