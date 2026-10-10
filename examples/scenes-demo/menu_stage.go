package main

import (
	"image/color"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/ui"
)

var menuColor = color.RGBA{R: 20, G: 20, B: 30, A: 255}

// NewMenuStage is a Stage with no world: its buttons are the engine's commands, Continue entering
// gameplay from its save.
func NewMenuStage(gameplay string) game.Stage {
	return stage.New(MenuStage).
		Scenes(func() []game.Scene { return []game.Scene{menuScene(gameplay)} }).
		Update(func(goke.RunCtx, time.Duration) {})
}

func menuScene(gameplay string) game.Scene {
	screen := ui.Layers(
		ui.Blank().Fill(menuColor),
		ui.Center(ui.Window("gram Stage/Scene demo",
			ui.Button("New game (Enter)", game.Enter{Stage: gameplay}),
			ui.Button("Continue", game.Load{Stage: gameplay}),
			ui.Button("Quit (Esc)", game.Quit{}),
		)),
	)
	return ui.NewScene(MenuScene, screen).Keys(
		control.Give(control.KeyPress{Key: control.KeyEnter}, "New game",
			game.Enter{Stage: gameplay}),
		control.Give(control.KeyPress{Key: control.KeyEscape}, "Quit", game.Quit{}),
	)
}
