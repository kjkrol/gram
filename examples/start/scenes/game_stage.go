package main

import (
	"github.com/kjkrol/gram/render"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	EntityCount = 12
	EntitySize  = 16
)

type gameplayArena struct {
	world   *world.Plugin
	motion  *motion.Plugin
	players *players.Plugin
	player  *players.Player
	cameras *cameras.Plugin
}

func NewGameplayStage() game.Stage {
	g := &gameplayArena{}
	return stage.New(GameplayStage).
		World(world.Config{
			Space: world.SpaceCfg{
				Width:  ScreenWidth,
				Height: ScreenHeight,
				Edges:  aabbworld.Torus,
			},
			Entities: world.EntitiesCfg{
				MaxCount: EntityCount,
				MinSize:  EntitySize,
				MaxSize:  EntitySize,
			},
		}).
		Plugins(g.makePlugins).
		Figures(g.defineFigures).
		Players(g.definePlayer).
		Spawn(g.spawnUnits).
		Scenes(g.defineScenes).
		Update(g.update)
}

func (g *gameplayArena) makePlugins(w *world.Plugin) {
	g.world = w
	g.motion = motion.NewPlugin(g.world)
	g.cameras = cameras.NewPlugin(g.world, cameras.Config{})
	g.players = players.NewPlugin(g.world, g.cameras)
}

func (g *gameplayArena) definePlayer() error {
	g.player = g.players.Local("player")
	return g.player.Bind(g.players.Defaults()...)
}

// mover drifts across the world, every one the same way
type mover struct{}

var _ world.Figure = mover{}

func (mover) Define(w *world.Plugin) {
	velocity := world.Velocity{}
	velocity.SetDelta(geom.NewVec(30, 20))
	kind.Define[world.Position](w.Kinds(), MoverKind, kind.Spec{
		comp.Load(func(p world.Position) world.Position { return p }),
		comp.Const(velocity),
	})
}

func (mover) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[world.Position](w.Kinds(), MoverKind), EntitySize, render.Solid(moverColor))
}

func (g *gameplayArena) defineFigures() { g.world.Figures(mover{}) }

func (g *gameplayArena) spawnUnits() {
	moverKind := kind.Named[world.Position](g.world.Kinds(), MoverKind)
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, EntitySize)
	entries := make([]kind.Entry, EntityCount)
	for i := range entries {
		entries[i] = moverKind.Entry(placement.Place(i, EntityCount))
	}
	g.world.Seed(entries...)
}

func (g *gameplayArena) defineScenes() []game.Scene {
	m := &gameplayScene{arena: g}
	return []game.Scene{ui.NewScene(WorldScene, m.screen()).
		Input(g.players.Handle).
		Issue(g.players.IssueAs(g.player)).
		Keys(
			control.Give(control.KeyPress{Key: control.KeyI}, "Panel",
				ui.Toggle{Name: PanelElement}),
			control.Give(control.KeyPress{Key: control.KeyM}, "Menu", game.Enter{Stage: MenuStage}),
		),
	}
}

func (g *gameplayArena) update(ctx goke.RunCtx, d time.Duration) {
	g.world.RunPlan(ctx, d)
	g.motion.RunPlan(ctx, d)
	g.cameras.RunPlan(ctx, d)
	g.players.RunPlan(ctx, d)
	ctx.Sync()
}
