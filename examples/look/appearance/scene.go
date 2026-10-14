package main

import (
	"image/color"

	"time"

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
	return ui.Layers(
		ui.Blank().Fill(backgroundColor),
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(s.players.Through(s.player)),
	)
}

func (s *arena) looks() *world.Atlas {
	walkerKind := kind.Named[walker](s.world.Kinds(), WalkerKind)
	ghostKind := kind.Named[walker](s.world.Kinds(), GhostKind)
	leaderKind := kind.Named[walker](s.world.Kinds(), LeaderKind)
	angry := s.world.Effects().Named(AngryEf)

	atlas := s.world.NewAtlas()
	// a walker is the figure, stepping the way it goes; angry, it stamps
	atlas.Add(walkerKind, heroHeight, heroStanding(0)).
		Facing(heroWays, heroStanding).
		Animated(heroWalkFrames, 120*time.Millisecond, heroPose(heroWalk)).WhileMoving().
		Under(angry,
			render.LoopedFacing(heroIdleFrames, 200*time.Millisecond, heroPose(heroIdle)))
	atlas.Add(leaderKind, Size, crowned(calmColor)).
		Under(angry, crowned(angryColor)).
		Turning()
	atlas.Add(ghostKind, Size, spook).
		Turning()
	atlas.Close()
	return atlas
}

// drawn east within the inscribed circle, so it turns whole
func crowned(body color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		render.Diamond(body)(dst, size)
		render.Arrow(90, 3, goldColor)(dst, size)
	}
}

func spook(dst *render.Canvas, size int) {
	r := float32(size) / 2
	dst.FillCircle(r, r, r-1, ghostColor)
	dst.FillRect(r-4, r-3, 2, 3, eyeColor)
	dst.FillRect(r+2, r-3, 2, 3, eyeColor)
}
