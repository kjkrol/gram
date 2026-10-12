package plugin

// Builtin marks a Plugin the engine installs itself; Use refuses it.
type Builtin interface{ Builtin() }
