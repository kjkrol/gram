package plugin

// Serializable is a tracked value giving Persistence the pointers it gob-encodes and decodes.
type Serializable interface {
	Persisted() []any
}
