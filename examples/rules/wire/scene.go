package main

import (
	_ "embed"
	"image/color"
	"time"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var (
	grassColor   = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	boardsColor  = color.RGBA{R: 120, G: 90, B: 55, A: 255}
	pitColor     = color.RGBA{R: 15, G: 12, B: 20, A: 255}
	plateColor   = color.RGBA{R: 160, G: 160, B: 170, A: 255}
	fenceColor   = color.RGBA{R: 85, G: 60, B: 40, A: 255}
	gateColor    = color.RGBA{R: 70, G: 75, B: 90, A: 255}
	gatewayColor = color.RGBA{R: 150, G: 130, B: 95, A: 255}
)

// four frames of 12 pixels in a row, the arm from the left (up) to the right (pulled down), the
// lever filling its frame; the first and the last from "Lever and Button" (OpenGameArt, CC0), the
// two between drawn after them
//
//go:embed lever.png
var leverSheet []byte

const (
	leverFrames = 4
	leverPixels = 12
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	heroKind := kind.Named[unitRow](s.world.Kinds(), HeroKind)
	standing := func(name string) kind.Of[standingRow] {
		return kind.Named[standingRow](s.world.Kinds(), name)
	}
	fx := s.world.Effects()
	lever := render.DecodeSheet("lever.png", leverSheet, leverPixels, leverPixels)

	worldAtlas := s.world.NewAtlas()
	worldAtlas.Add(standing(TrapdoorKind), CellSize, render.Solid(boardsColor)).
		Under(fx.Named(OpenEf), render.Solid(pitColor))
	worldAtlas.Add(standing(GateKind), CellSize, render.Solid(gateColor)).
		Under(fx.Named(AjarEf), render.Solid(gatewayColor))
	worldAtlas.Add(standing(PlateKind), EntitySize, render.Solid(plateColor))
	worldAtlas.Add(standing(LeverKind), LeverSize, lever.At(0, 0)).
		Under(fx.Named(LeverPulledEf), lever.At(leverFrames-1, 0)).
		Under(fx.Named(LeverPullingEf), render.Phased(leverFrames,
			func(i int) render.SpriteDrawer { return lever.At(i, 0) })).
		Under(fx.Named(LeverPushingEf), render.Phased(leverFrames,
			func(i int) render.SpriteDrawer { return lever.At(leverFrames-1-i, 0) }))
	worldAtlas.Add(heroKind, heroHeight, heroStanding(0)).
		Facing(heroWays, heroStanding).
		Animated(heroWalkFrames, 120*time.Millisecond, heroPose(heroWalk)).WhileMoving().
		Under(fx.Named(BrakingEf),
			render.LoopedFacing(heroBrakeFrames, 150*time.Millisecond, heroPose(heroBrake))).
		Under(fx.Named(BlinkingEf), render.TimedFacing(heroBlinkBeat, heroBeat)).
		Under(fx.Named(StampingEf), render.TimedFacing(heroBlinkBeat, heroBeat)).
		Under(fx.Named(StampEf), render.PhasedFacing(heroStampFrames, heroStampSeries)).
		Under(fx.Named(LookingEf), render.TimedFacing(heroBlinkBeat, heroBeat)).
		Under(fx.Named(PullingEf),
			render.PhasedFacing(heroPullFrames, heroPose(heroPull)))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(PitCell, render.Solid(pitColor))
	boardAtlas.Add(FenceCell, render.Solid(fenceColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.board.Renderer(),
		s.world.Renderer(), s.collision.Renderer(),
		s.selection.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	feed := render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{Follow: HeroName}),
		m.picture())
	return ui.Layers(
		ui.Image(feed).Input(s.players.Through(s.player)),
		ui.Label("Space: pull the lever").Under(s.world.Effects().Named(InReachEf)),
	)
}
