package main

import (
	"image/color"
	"log"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/render"
)

// =========================== Stage ===========================

// Menu is the splash/menu — no entities, no gameplay plugins: a Stage of one scene.
type Menu struct {
	game.Stage // defined in sections: NewMenuStage
}

// NewMenuStage builds the Menu that switches to the Stage named gameplayName on start.
func NewMenuStage(gameplayName string) *Menu {
	return &Menu{Stage: stage.New(MenuStage).
		Scenes(func() []game.Scene { return []game.Scene{&menuScene{gameplayName: gameplayName}} }).
		Update(func(goke.RunCtx, time.Duration) {})}
}

// =========================== Scene ===========================

// menuColor is the menu's backdrop.
var menuColor = color.RGBA{R: 20, G: 20, B: 30, A: 255}

// menuScene shows the splash text, enters the gameplay Stage on Enter and quits on Escape.
type menuScene struct{ gameplayName string }

var _ game.Scene = (*menuScene)(nil)

func (m *menuScene) Name() string { return MenuScene }

func (m *menuScene) Layers() []render.Layer { return []render.Layer{&menuRenderer{}} }

func (m *menuScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		switch k.Key {
		case control.KeyEnter:
			if err := runtime.SwitchStage(m.gameplayName); err != nil {
				log.Printf("switch stage: %v", err)
			}
		case control.KeyEscape:
			runtime.Quit()
		}
	}
}

func (m *menuScene) Focusable() bool { return true }

type menuRenderer struct{}

func (r *menuRenderer) Init(*goke.SysInit) {}

func (r *menuRenderer) Draw(screen *render.Image) {
	screen.Fill(menuColor)
	render.DebugPrintAt(screen, "gram Stage/Scene demo\n\nPress ENTER to start", 20, 20)
}
