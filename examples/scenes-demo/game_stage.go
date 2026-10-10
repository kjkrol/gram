package main

import (
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	EntityCount = 12
	EntitySize  = 16

	saveBasePath = "scenes-demo"
)

type gameplayArena struct {
	world   *world.Plugin
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
		Players(g.definePlayer).
		Kinds(g.defineKinds).
		Restore(g.restore).
		Spawn(g.spawnUnits).
		Scenes(g.defineScenes).
		Update(g.update)
}

func (g *gameplayArena) makePlugins(w *world.Plugin) {
	g.world = w
	g.cameras = cameras.NewPlugin(g.world)
	g.players = players.NewPlugin(g.world, g.cameras).WithSaves(saveBasePath)
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

func (g *gameplayArena) defineScenes() []game.Scene {
	m := &gameplayScene{arena: g}
	return []game.Scene{ui.NewScene(WorldScene, m.screen()).
		Input(g.players.Handle).
		Issue(g.players.IssueAs(g.player)).
		Keys(control.Give(control.KeyPress{Key: control.KeyP}, "Panel",
			ui.Toggle{Name: PanelElement})),
	}
}

// TODO: ten kod to copy-paste
func (g *gameplayArena) restore(p game.Persistence) (bool, error) {
	saves, err := p.List(saveBasePath)
	if err != nil {
		return false, err
	}
	if !slices.Contains(saves, "") {
		return false, nil
	}
	if err := p.Load(saveBasePath, ""); err != nil {
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
