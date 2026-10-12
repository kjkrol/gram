package plugin

// Made are the plugins made for a Stage, in the order made, which is the order the Stage uses
// them in.
type Made struct{ plugins []Plugin }

// Add notes p as made for the Stage.
func (m *Made) Add(p Plugin) { m.plugins = append(m.plugins, p) }

// All are the plugins made, in the order made.
func (m *Made) All() []Plugin { return m.plugins }
