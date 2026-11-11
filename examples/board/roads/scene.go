package main

import (
	"fmt"
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

type mainScene struct {
	arena *arena
}

var (
	grassColor  = color.RGBA{R: 95, G: 140, B: 70, A: 255}
	hillColor   = color.RGBA{R: 140, G: 125, B: 95, A: 255}
	forestColor = color.RGBA{R: 30, G: 85, B: 45, A: 255}
	waterColor  = color.RGBA{R: 40, G: 90, B: 170, A: 255}
	fordColor   = color.RGBA{R: 85, G: 135, B: 175, A: 255}
	roadColor   = color.RGBA{R: 165, G: 135, B: 95, A: 255}
	deckColor   = color.RGBA{R: 115, G: 85, B: 55, A: 255}
	railColor   = color.RGBA{R: 70, G: 50, B: 35, A: 255}
	pierColor   = color.RGBA{R: 160, G: 150, B: 135, A: 255}
	greenColor  = color.RGBA{R: 120, G: 170, B: 85, A: 255}
	wellColor   = color.RGBA{R: 60, G: 60, B: 70, A: 255}
	roofColor   = color.RGBA{R: 180, G: 80, B: 55, A: 255}
	roofShade   = color.RGBA{R: 135, G: 55, B: 40, A: 255}
	panelColor  = color.RGBA{R: 20, G: 20, B: 20, A: 180}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	a.light.WithDistant(sky.DefaultSun)
	a.topography.
		Style(WaterCell, painter.Style{Shine: 0.9}).
		Style(FordCell, painter.Style{Shine: 0.5})

	cam := a.cameras.Write().New(cameras.FromAbove, camera.Config{})
	return ui.Layers(
		ui.Image(render.NewFeed(cam, a.world.Read(0).Picture())).
			Input(a.players.Through(a.player)),
		ui.Drawer(a.world.Read(0).Clock().HUD()),
		ui.TopRight(ui.LabelOf(&status{arena: a}).Padding(6).Fill(panelColor).Passing()).
			Margin(8),
		ui.BottomMiddle(a.roads.Reason().Padding(6).Fill(panelColor)).Margin(40),
		ui.BottomMiddle(a.bridges.Reason().Padding(6).Fill(panelColor)).Margin(40),
		ui.BottomMiddle(a.tunnels.Reason().Padding(6).Fill(panelColor)).Margin(40),
		ui.Center(a.roads.Window()),
		ui.Center(a.bridges.Window()),
		ui.Center(a.tunnels.Window()),
	)
}

// read several times a frame: worded anew only as the treasury or the world's state changes
type status struct {
	arena         *arena
	gold          float32
	winter, works bool
	text          string
}

func (s *status) Text(uid.UID64, bool) (string, bool) {
	gold, winter, works := s.arena.treasury.Gold, s.under(WinterEf), s.under(RoadworksEf)
	if s.text != "" && gold == s.gold && winter == s.winter && works == s.works {
		return s.text, true
	}
	s.gold, s.winter, s.works = gold, winter, works
	s.text = fmt.Sprintf("Treasury: %.0f gold", gold)
	if winter {
		s.text += "   Winter: the ground is frozen"
	}
	if works {
		s.text += "   Roadworks"
	}
	return s.text, true
}

func (s *status) under(name string) bool {
	w := s.arena.world
	return w.Read(0).Effects().Has(w.Entity(), w.Write().Effects().Named(name))
}
