package plugin

// Made are the plugins made for a Stage, in the order made: the Stage uses them in that order. A
// plugin's constructor adds itself — over a world through world.NewSelf, else by Add.
type Made struct{ plugins []Plugin }

// Add notes p as made for the Stage, after those made before it.
func (m *Made) Add(p Plugin) { m.plugins = append(m.plugins, p) }

// All are the plugins made, in the order made.
func (m *Made) All() []Plugin { return m.plugins }
