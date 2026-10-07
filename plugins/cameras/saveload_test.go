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

// savedStage is a world and its cameras, loaded from loadFrom when set.
type savedStage struct {
	cams     *cameras.Plugin
	cam      camera.Camera
	loadFrom string
	stack    game.Scenes
}

func (g *savedStage) Name() string { return "stage" }
func (g *savedStage) Init(ctx game.Initializer) error {
	g.cams = cameras.NewPlugin(ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}))
	g.cam = g.cams.New(cameras.TopDown(), camera.Config{})
	return ctx.Use(g.cams)
}
func (g *savedStage) Restore(p game.Persistence) (bool, error) {
	if g.loadFrom == "" {
		return false, nil
	}
	if err := p.Load(g.loadFrom, ""); err != nil {
		return false, err
	}
	return true, nil
}
func (g *savedStage) Spawn() error                      { return nil }
func (g *savedStage) Update(goke.RunCtx, time.Duration) {}
func (g *savedStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// oneStage is a game of one Stage.
type oneStage struct{ stage game.Stage }

func (g oneStage) Props() game.Props { return game.Props{} }
func (g oneStage) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

// A game saved and loaded through the engine shows what its main camera showed.
func TestPlugin_SaveLoad_CameraRoundTrip(t *testing.T) {
	basePath := t.TempDir() + "/save"

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
	if err := eng.Persistence().Save(basePath, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	g2 := &savedStage{loadFrom: basePath}
	if err := engine.NewEngine(oneStage{stage: g2}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := g2.cam.Bounds(); got != wantBounds {
		t.Errorf("Bounds() after Load = %+v, want %+v", got, wantBounds)
	}
	if got := g2.cam.Zoom(); got != wantZoom {
		t.Errorf("Zoom() after Load = %v, want %v", got, wantZoom)
	}
}
