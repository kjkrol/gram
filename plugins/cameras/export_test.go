package cameras

// Redraw is the adapter's step applied: every picture at factor times its resolution.
func (p *Plugin) Redraw(factor float32) { p.redraw(factor) }
