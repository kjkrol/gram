package main

import (
	"image/color"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

const (
	EntityCount = 12
	EntitySize  = 16

	saveBasePath = "scenes-demo"
)

// =========================== Stage ===========================

// gameplayArena collects the gameplay Stage's plugins — the real game, its own fresh ECS, built
// only once entered from the menu — and every section of the Stage builds on it.
type gameplayArena struct {
	world   *world.Plugin
	players *players.Plugin
	player  *players.Player // the one at the keyboard
	cameras *cameras.Plugin
	scene   *ui.Scene // the one scene: the world, the hud, the panel over them

	// SaveBasePath overrides where saves are read/written; tests set this to a temp path.
	SaveBasePath string
}

// NewGameplayStage makes the gameplay arena and defines the Stage on it, a section at a time;
// saveBasePath overrides where its saves are read and written, empty for the demo's own.
func NewGameplayStage(saveBasePath string) (*gameplayArena, game.Stage) {
	g := &gameplayArena{SaveBasePath: saveBasePath}
	return g, stage.New(GameplayStage).
		Plugins(g.usePlugins).
		Players(g.definePlayer).
		Kinds(g.defineKinds).
		Restore(g.restore).
		Spawn(g.spawnUnits).
		Scenes(g.defineScenes).
		Update(g.update)
}

func (g *gameplayArena) basePath() string {
	if g.SaveBasePath != "" {
		return g.SaveBasePath
	}
	return saveBasePath
}

func (g *gameplayArena) usePlugins(ctx game.Initializer) error {
	g.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: EntityCount, MinSize: EntitySize, MaxSize: EntitySize},
	})
	g.cameras = cameras.NewPlugin(g.world)
	g.players = players.NewPlugin(g.world, g.cameras).WithSaves(g.basePath())
	if err := ctx.Use(g.cameras); err != nil {
		return err
	}
	return ctx.Use(g.players)
}

func (g *gameplayArena) definePlayer() error {
	g.player = g.players.Local("player")
	return g.player.Bind(g.players.Defaults()...)
}

func (g *gameplayArena) defineKinds() {
	velocity := world.Velocity{}
	velocity.SetDelta(geom.NewVec(30, 20))
	kind.Define[world.Position](g.world.Kinds(), MoverKind, kind.Spec{
		comp.Load(func(p world.Position) world.Position { return p }),
		comp.Const(velocity),
	})
}

// defineScenes is the one scene: P opens and closes the panel, the rest of the keys are the
// player's.
func (g *gameplayArena) defineScenes() []game.Scene {
	g.scene = ui.NewScene(WorldScene, g.screen()).
		Input(g.players.Handle).
		Issue(g.players.IssueAs(g.player)).
		Keys(control.Give(control.KeyPress{Key: control.KeyP}, "Panel", ui.Toggle{Name: PanelElement}))
	return []game.Scene{g.scene}
}

func (g *gameplayArena) restore(p game.Persistence) (bool, error) {
	saves, err := p.List(g.basePath())
	if err != nil {
		return false, err
	}
	if !slices.Contains(saves, "") {
		return false, nil
	}
	if err := p.Load(g.basePath(), ""); err != nil {
		return false, err
	}
	return true, nil
}

func (g *gameplayArena) spawnUnits() {
	moverKind := kind.Named[world.Position](g.world.Kinds(), MoverKind)
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, EntitySize)
	entries := make([]kind.Entry, EntityCount)
	for i := range entries {
		entries[i] = moverKind.Entry(placement.Place(i, EntityCount))
	}
	g.world.Seed(entries...)
}

func (g *gameplayArena) update(ctx goke.RunCtx, d time.Duration) {
	g.world.RunPlan(ctx, d)
	g.cameras.RunPlan(ctx, d)
	g.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

// The scene's colours: the movers and the backdrop.
var (
	moverColor      = color.RGBA{R: 90, G: 200, B: 110, A: 255}
	backgroundColor = color.RGBA{R: 30, G: 30, B: 40, A: 255}
)

// picture dresses the movers and hands the world's picture.
func (g *gameplayArena) picture() render.Picture {
	moverKind := kind.Named[world.Position](g.world.Kinds(), MoverKind)
	atlas := render.NewAtlas()
	atlas.Add(moverKind, EntitySize, render.Solid(moverColor))
	atlas.Close()
	g.world.WithRenderer(atlas)
	return render.NewComposer(g.world.Renderer())
}

// screen is the world through a camera of its own on a backdrop, the player's view, the keys at the
// bottom, and the panel: a modal window over it all, hidden until P — while it is shown, the world
// keeps ticking behind it and takes no click.
func (g *gameplayArena) screen() *ui.Element {
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Blank().Fill(backgroundColor),
		ui.Image(render.NewFeed(g.cameras.New(cameras.TopDown(), camera.Config{}), g.picture())).Input(g.players.Through(g.player)),
		ui.BottomLeft(ui.Label("P: panel, F5: save, K: keys")).Margin(8),
		ui.Center(ui.Window("PANEL",
			ui.Label("the world keeps ticking behind me"),
			ui.Button("Close (or P)", ui.Hide{Name: PanelElement}),
		)).Named(PanelElement).Hidden().Modal(),
	)
}
