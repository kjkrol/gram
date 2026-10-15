package board

import "github.com/kjkrol/gram/render"

// HoldsView is what draws the cells held.
func (p *Plugin) HoldsView() render.Source { return p.holdsView }
