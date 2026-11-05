package order

import "github.com/kjkrol/gram/plugins/world/sector"

// Aiming marks a unit whose order is aimed at an aggregate (MoveOrder.Aimed), or was till its
// order ended: the units over which the orders aimed at each aggregate are counted.
type Aiming struct{}

// Sought is on an aggregate kept ticked at L3 while orders are aimed at it: the Keen it carried
// of its own as the first came, if it Had one, put back as the last ends.
type Sought struct {
	Keen sector.Keen
	Had  bool
}
