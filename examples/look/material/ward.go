package main

import (
	"embed"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const wardPx = 9 * CellSize

//go:embed ward.wgsl
var wardFS embed.FS

// registered before the composer's shader compiles
var wardMaterial = render.RegisterMaterials(render.Files(wardFS, "ward.wgsl"), nil, "Ward")[0]

type wardRow struct {
	At geom.Vec `yaml:"at"`
}

// ward is one standing entity drawn by a material of the game's own WGSL; calmed, a plain
// sprite for a while.
type ward struct{}

var (
	_ board.Figure   = ward{}
	_ board.Stateful = ward{}
)

func (ward) Effects(b *board.Plugin) {
	b.Read(0).World().Effects().Define(CalmEf, effect.Spec{
		effect.Described("Calmed: the ward's glow put out for a while."),
		effect.Lasts(2 * time.Second),
	})
}

func (ward) Define(b *board.Plugin) {
	kind.Define[wardRow](b.Read(0).World().Kinds(), WardKind, kind.Spec{
		comp.Load(func(w wardRow) world.Position {
			return world.Position{
				AABB: plane.NewAABB(geom.NewVec(w.At.X-wardPx/2, w.At.Y-wardPx/2), wardPx, wardPx),
			}
		}),
		comp.Const(world.Velocity{}),
	})
}

func (ward) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[wardRow](w.Kinds(), WardKind), wardPx, wardMaterial).
		Under(w.Effects().Named(CalmEf), render.Dot(8, ashColor))
}
