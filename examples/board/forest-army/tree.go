package main

import (
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/forest"
	"github.com/kjkrol/gram/plugins/world"
)

const (
	treePx     = 20
	treeHeight = 30
)

// tree is the forest's one kind of tree, a post nobody walks through: a soldier striking it fells
// it.
var tree = forest.Spec{
	Height: treeHeight,
	Crown:  treePx,
	Trunk:  0.3,
	Colours: forest.Colours{
		Crown:  crownColor,
		Leaves: leafColor,
		Trunk:  trunkColor,
	},
	Solid: true,
	Body:  true,
	Roles: []string{TreeRole},
}

// felled is the tree's own rule, its folding in with its sector the forest plugin's
func felled(w *world.Plugin) act.Rule {
	return act.Branch[collision.Meeting]{
		Name: "felled",
		When: cond.Other.If(w.Roles().Named(FellerRole)),
		Then: act.Self.Do(world.Despawn{}),
	}
}
