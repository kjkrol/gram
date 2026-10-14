package main

import (
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// ghost is a pale disc with two eyes, turned the way it goes.
type ghost struct{}

var _ world.Figure = ghost{}

func (ghost) Define(w *world.Plugin) { kind.Define[walker](w.Kinds(), GhostKind, walkerSpec(w)) }

func (ghost) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[walker](w.Kinds(), GhostKind), Size, spook).Turning()
}

func spook(dst *render.Canvas, size int) {
	r := float32(size) / 2
	dst.FillCircle(r, r, r-1, ghostColor)
	dst.FillRect(r-4, r-3, 2, 3, eyeColor)
	dst.FillRect(r+2, r-3, 2, 3, eyeColor)
}
