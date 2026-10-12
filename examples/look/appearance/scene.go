package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var (
	calmColor       = color.RGBA{R: 70, G: 130, B: 220, A: 255}
	angryColor      = color.RGBA{R: 220, G: 60, B: 50, A: 255}
	noseColor       = color.RGBA{R: 245, G: 245, B: 230, A: 255}
	ghostColor      = color.RGBA{R: 225, G: 230, B: 245, A: 200}
	eyeColor        = color.RGBA{A: 255}
	goldColor       = color.RGBA{R: 240, G: 200, B: 40, A: 255}
	backgroundColor = color.RGBA{R: 40, G: 44, B: 52, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	s.world.WithRenderer(s.looks())
	return render.NewComposer(s.world.Renderer())
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Blank().Fill(backgroundColor),
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(s.players.Through(s.player)),
	)
}

// looks declares every look in one place, on the world's atlas: the walker by the twin of the
// way it goes (Facing) and a plain red square while angry — anger has no direction; the leader
// turned smoothly with his crown on (Turning), red and crowned while angry; the ghost a ghost
// whatever happens.
func (s *arena) looks() *world.Atlas {
	walkerKind := kind.Named[walker](s.world.Kinds(), WalkerKind)
	ghostKind := kind.Named[walker](s.world.Kinds(), GhostKind)
	leaderKind := kind.Named[walker](s.world.Kinds(), LeaderKind)
	angry := s.world.Effects().Named(AngryEf)

	atlas := s.world.NewAtlas()
	atlas.Add(walkerKind, Size, nosed(calmColor)).
		Facing(4, func(angle float64) render.SpriteDrawer { return nosedAt(angle, calmColor) }).
		Under(angry, render.Solid(angryColor))
	atlas.Add(leaderKind, Size, crowned(calmColor)).
		Under(angry, crowned(angryColor)).
		Turning() // the crown turns with him — and while he is angry too
	atlas.Add(ghostKind, Size, spook).
		Turning() // a ghost drifts face first; anger leaves it unmoved: no look under it
	atlas.Close()
	return atlas
}

// nosed is a square of body with a light nose eastwards — the way angle 0 points.
func nosed(body color.RGBA) render.SpriteDrawer { return nosedAt(0, body) }

// nosedAt is a square of body with its nose on the side angleDeg faces.
func nosedAt(angleDeg float64, body color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		render.Solid(body)(dst, size)
		render.Arrow(angleDeg, 4, noseColor)(dst, size)
	}
}

// crowned is a diamond of body under a gold band — drawn east first, within the circle inscribed
// in the box, so it turns whole.
func crowned(body color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		render.Diamond(body)(dst, size)
		render.Arrow(90, 3, goldColor)(dst, size)
	}
}

// spook is the ghost: a pale circle with two eyes.
func spook(dst *render.Canvas, size int) {
	r := float32(size) / 2
	dst.FillCircle(r, r, r-1, ghostColor)
	dst.FillRect(r-4, r-3, 2, 3, eyeColor)
	dst.FillRect(r+2, r-3, 2, 3, eyeColor)
}
