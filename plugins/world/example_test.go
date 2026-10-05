package world_test

import (
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world"
)

func ExamplePlugin_Seed() {
	plugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 800, Height: 600},
		Entities: world.EntitiesCfg{MaxCount: 10, MinSize: 8, MaxSize: 8},
	})
	placement := world.NewGridPlacement(800, 600, 8)

	kind.Define[world.Position](plugin.Kinds(), "dot", kind.Spec{
		comp.Load(func(p world.Position) world.Position { return p }),
		comp.Const(world.Velocity{}),
	})
	dot := kind.Named[world.Position](plugin.Kinds(), "dot")

	for i := range 10 {
		plugin.Seed(dot.Entry(placement.Place(i, 10)))
	}

	if err := plugin.Populate(); err != nil {
		panic(err)
	}
}
