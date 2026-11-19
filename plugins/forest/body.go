package forest

import "github.com/kjkrol/gram/plugins/world"

// The paints of a tree's body, which a Part of its Spec names: Leaves, in its Colours' Crown,
// dotted with their Leaves and with what it Bears, and Wood, in their Trunk.
const (
	Leaves = iota
	Wood
)

// body is the tree's body as the world stands it: its Parts within its crown's width and its
// height, in its Colours
func (s Spec) body() world.Body {
	return world.Body{
		Wide:  s.Crown,
		Tall:  s.Height,
		Parts: s.Parts,
		Paints: []world.Paint{
			Leaves: {Colour: s.Colours.Crown, Lit: s.Colours.Leaves, Dots: s.Colours.Bears},
			Wood:   {Colour: s.Colours.Trunk},
		},
	}
}
