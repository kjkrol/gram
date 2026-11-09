package main

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// a band's walkers stand in so many files across and ranks down
const (
	bandFiles = 6
	bandRanks = 5
)

// where a band's walkers stand as it unfolds: a cell apart in its files and ranks, round its
// centre
var inBlock = func() (at [bandFiles * bandRanks]geom.Vec) {
	for i := range at {
		at[i] = geom.NewVec((float32(i%bandFiles)-(bandFiles-1)/2.0)*CellSize,
			(float32(i/bandFiles)-(bandRanks-1)/2.0)*CellSize)
	}
	return at
}()

type bandRow struct {
	Start cell.ID `yaml:"start"`
}

// band is a party of the plateau's walkers seen from afar, one point: come down to L3 it stands
// them a cell apart round it, each taking its way over, and folded it takes the way back from
// them.
type band struct {
	color color.RGBA
	// what its walkers are told as they are made: whose they are, each picked itself
	joined func() []command.Command
}

var (
	_ board.Figure = band{}
	_ board.Ruled  = band{}
)

func (bd band) Rules(b *board.Plugin) {
	w := b.Read(0).World()
	spread := make([]act.Step, 0, len(inBlock))
	for _, at := range inBlock {
		part := kind.EntryOf(PlateauKind, unitRow{}).Told(navigation.Inherit{}).
			Told(bd.joined()...)
		spread = append(spread, act.Self.Do(world.SpawnPart{Entry: part, At: at}))
	}
	w.Roles().Define(BandRole,
		act.Branch[world.Unfold]{
			Name: "spread",
			When: cond.Self.If(cond.Always),
			Then: act.Steps(spread...),
		},
		act.Branch[world.Fold]{
			Name: "gather",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(navigation.Inherit{}),
		})
}

func (bd band) Define(b *board.Plugin) {
	w := b.Read(0).World()
	// its middle on the west side of its start cell, so its six files stand on cells
	units := board.NewUnits[bandRow](b, board.Shape{Size: scale.Figure(0.6), Height: figure},
		func(r bandRow) geom.Vec {
			return b.Read(0).Board().CellCenter(r.Start).Sub(geom.NewVec(CellSize/2, 0))
		})
	units.Define(BandKind, unit.Mover{Domain: cell.Land | HighlandDomain}, walking,
		comp.Const(world.Aggregate{Strength: bandFiles * bandRanks}),
		// its walkers stand round it, pushed by nothing of its own
		comp.Without[collision.Collider](),
		comp.Without[collision.Physics](),
		ceremony.Plays(w.Roles().Named(BandRole)))
}

// unseen while its walkers stand, five figures on a disc from L4
func (bd band) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[bandRow](w.Kinds(), BandKind), iconPx, unseen).
		At(lod.L4, bandIcon(bd.color)).
		Styled(world.BannerStyle{Fill: bd.color}).
		Styled(selection.Highlight{Line: bd.color, Width: 2}).
		Styled(minimap.Mark{Color: bd.color, Size: 6})
}

// bandIcon is five figures in c on a pale disc, two behind three.
func bandIcon(c color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		k := float32(size) / iconPx
		disc(dst, k)
		silhouette(dst, k, 9, 15, 0.8, c, false)
		silhouette(dst, k, 15, 15, 0.8, c, false)
		silhouette(dst, k, 6, 20, 0.9, c, false)
		silhouette(dst, k, 12, 20, 0.9, c, false)
		silhouette(dst, k, 18, 20, 0.9, c, false)
	}
}
