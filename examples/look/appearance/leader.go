package main

import (
	"image/color"

	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// leader is a crowned diamond turned the way it goes, red while angry.
type leader struct{}

var _ world.Figure = leader{}

func (leader) Define(w *world.Plugin) { kind.Define[walker](w.Kinds(), LeaderKind, walkerSpec(w)) }

func (leader) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[walker](w.Kinds(), LeaderKind), Size, crowned(calmColor)).
		Under(w.Effects().Named(AngryEf), crowned(angryColor)).
		Turning()
}

// drawn east within the inscribed circle, so it turns whole
func crowned(body color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		render.Diamond(body)(dst, size)
		render.Arrow(90, 3, goldColor)(dst, size)
	}
}
