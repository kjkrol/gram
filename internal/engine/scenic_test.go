package engine

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
)

type plainScene struct{ name string }

func (s plainScene) Name() string                                                    { return s.name }
func (plainScene) Layers() []render.Layer                                            { return nil }
func (plainScene) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}
func (plainScene) Focusable() bool                                                   { return true }

type scenicPlugin struct{ stubPlugin }

func (scenicPlugin) Scenes() []game.Scene { return []game.Scene{plainScene{"plugin.list"}} }

func TestStage_HasTheScenesOfItsPlugins(t *testing.T) {
	st := stage.New("meadow").
		Plugins(func(made *plugin.Made) { made.Add(&scenicPlugin{stubPlugin{name: "test.scenic"}}) }).
		Scenes(func() []game.Scene { return []game.Scene{plainScene{"main"}} }).
		Update(func(goke.RunCtx, time.Duration) {})
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	var names []string
	for _, sc := range st.Stack().All() {
		names = append(names, sc.Name())
	}
	if !slices.Equal(names, []string{"main", "plugin.list"}) {
		t.Errorf("the stack holds %v, want the game's scene, then the plugin's", names)
	}
	comp := st.Stack().Composition()
	if got := comp.Order(); !slices.Equal(got, []string{"main"}) {
		t.Errorf("shown at the start: %v, want the game's first scene alone", got)
	}
	comp.Show("plugin.list")
	if comp.Active() != "plugin.list" {
		t.Errorf("shown, the plugin's scene is not on top: active %q", comp.Active())
	}
}
