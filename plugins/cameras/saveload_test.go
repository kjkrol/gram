package cameras_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
)

type savedStage struct {
	cams  *cameras.Plugin
	cam   camera.Camera
	load  bool
	stack game.Scenes
}

func (g *savedStage) Name() string { return "stage" }
func (g *savedStage) Init(ctx game.Initializer) error {
	g.cams = cameras.NewPlugin(ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}), cameras.Config{})
	g.cam = g.cams.New(cameras.FromAbove, camera.Config{})
	return ctx.Use(g.cams)
}
func (g *savedStage) Loaded() error                     { return nil }
func (g *savedStage) Spawn() error                      { return nil }
func (g *savedStage) Update(goke.RunCtx, time.Duration) {}
func (g *savedStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

type oneStage struct{ stage game.Stage }

func (g oneStage) Props() game.Props { return game.Props{} }
func (g oneStage) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

func TestPlugin_SaveLoad_CameraRoundTrip(t *testing.T) {
	t.Chdir(t.TempDir())

	g := &savedStage{}
	eng := engine.NewEngine(oneStage{stage: g})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	cam := g.cam
	cam.MoveTo(100, 150)
	b := cam.Bounds()
	cam.ZoomIn(2, float32(b.TopLeft.X+b.BottomRight.X)/2, float32(b.TopLeft.Y+b.BottomRight.Y)/2)
	wantBounds, wantZoom := cam.Bounds(), cam.Zoom()
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	g2 := &savedStage{load: true}
	eng2 := engine.NewEngine(oneStage{stage: g2})
	if err := eng2.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := g2.cam.Bounds(); got != wantBounds {
		t.Errorf("Bounds() after Load = %+v, want %+v", got, wantBounds)
	}
	if got := g2.cam.Zoom(); got != wantZoom {
		t.Errorf("Zoom() after Load = %v, want %v", got, wantZoom)
	}
}
