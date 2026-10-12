package entity

// Block marks an entity standing still as a solid whose shadow the ground bakes with its own: one
// lying flat (a bridge's deck) over the rectangle its [Lie] says, from its Altitude down its Z's
// Height; any other (a pier) over its box from its Altitude up its Height.
type Block struct{}
