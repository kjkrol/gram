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
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type plainScene struct{ name string }

func (s plainScene) Name() string                                                    { return s.name }
func (plainScene) Drawers() []render.Renderer                                        { return nil }
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

func TestStage_WithAWorldHasATelemetryPanelHiddenUnlessTheGameMadeOne(t *testing.T) {
	cfg := world.Config{Space: world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}}
	for name, made := range map[string]func(w *world.Plugin) *telemetry.Plugin{
		"none":    func(*world.Plugin) *telemetry.Plugin { return nil },
		"its own": func(w *world.Plugin) *telemetry.Plugin { return telemetry.NewPlugin(w) },
	} {
		t.Run(name, func(t *testing.T) {
			var own *telemetry.Plugin
			var theWorld *world.Plugin
			st := stage.New("meadow").World(cfg).
				Plugins(func(w *world.Plugin) { theWorld, own = w, made(w) }).
				Scenes(func() []game.Scene { return []game.Scene{plainScene{"main"}} }).
				Update(func(goke.RunCtx, time.Duration) {})
			if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
				t.Fatalf("Init: %v", err)
			}
			var names []string
			for _, sc := range st.Stack().All() {
				names = append(names, sc.Name())
			}
			if !slices.Contains(names, telemetry.SceneName) {
				t.Errorf("the stack holds %v, want the telemetry panel among them", names)
			}
			if got := st.Stack().Composition().Order(); !slices.Equal(got, []string{"main"}) {
				t.Errorf("shown at the start: %v, want the game's scene alone", got)
			}
			if p := telemetry.Of(theWorld); p == nil || (own != nil && p != own) {
				t.Errorf("the world's telemetry is %p, want %p (the game's own where it made one)",
					p, own)
			}
		})
	}
}

func TestStage_WithPlayersHasTheirControlsAndSettings(t *testing.T) {
	cfg := world.Config{Space: world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10}}
	st := stage.New("meadow").World(cfg).
		Plugins(func(w *world.Plugin) {
			players.NewPlugin(w, cameras.NewPlugin(w, cameras.Config{}))
		}).
		Scenes(func() []game.Scene { return []game.Scene{plainScene{"main"}} }).
		Update(func(goke.RunCtx, time.Duration) {})
	if err := NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	var names []string
	for _, sc := range st.Stack().All() {
		names = append(names, sc.Name())
	}
	if !slices.Contains(names, players.ControlsName) || !slices.Contains(names, players.SettingsName) {
		t.Errorf("the stack holds %v, want the players' controls and settings among them", names)
	}
}
