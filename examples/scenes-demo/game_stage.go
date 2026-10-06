package main

import (
	"fmt"
	"image/color"
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
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
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
	panel   *panelScene
	stage   game.Stage // the Stage defined on this arena: the hud reads its Composition

	// SaveBasePath overrides where saves are read/written; tests set this to a temp path.
	SaveBasePath string
}

// NewGameplayStage makes the gameplay arena and defines the Stage on it, a section at a time;
// saveBasePath overrides where its saves are read and written, empty for the demo's own.
func NewGameplayStage(saveBasePath string) (*gameplayArena, game.Stage) {
	g := &gameplayArena{SaveBasePath: saveBasePath}
	g.stage = stage.New("gameplay").
		Plugins(g.usePlugins).
		Players(g.definePlayer).
		Kinds(g.defineKinds).
		Scenes(g.defineScenes).
		Shows("world", "hud").
		Restore(g.restore).
		Units(g.placeUnits).
		Update(g.update)
	return g, g.stage
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
	g.players = players.NewPlugin(g.world).WithSaves(g.basePath())
	return ctx.Use(g.players)
}

func (g *gameplayArena) definePlayer() error {
	return g.players.Local("player").Bind(g.players.Defaults()...)
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
	g.panel = &panelScene{arena: g}
	return []game.Scene{&worldScene{arena: g}, g.panel, &hudScene{arena: g}}
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

func (g *gameplayArena) placeUnits() {
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, EntitySize)
	entries := make([]kind.Entry, EntityCount)
	for i := range entries {
		entries[i] = kind.Named[world.Position](g.world.Kinds(), MoverKind).Entry(placement.Place(i, EntityCount))
	}
	g.world.Seed(entries...)
}

func (g *gameplayArena) update(ctx goke.RunCtx, d time.Duration) {
	g.world.RunPlan(ctx, d)
	g.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

// worldScene draws the moving entities — always visible, and active
// whenever the panel isn't shown. P opens the panel.
type worldScene struct{ arena *gameplayArena }

var _ game.Scene = (*worldScene)(nil)

func (w *worldScene) Name() string { return "world" }

func (w *worldScene) Layers() []render.Layer {
	s := w.arena

	atlas := render.NewAtlas()
	atlas.Add(kind.Named[world.Position](s.world.Kinds(), MoverKind).SpriteID(), EntitySize, render.Solid(color.RGBA{R: 90, G: 200, B: 110, A: 255}))
	atlas.Close()
	s.world.WithRenderer(atlas)

	return []render.Layer{
		render.NewCachedRenderer(render.SolidBackground{Color: color.RGBA{R: 30, G: 30, B: 40, A: 255}}, ScreenWidth, ScreenHeight),
		render.NewComposer(s.world.Renderer()),
	}
}

// Viewports are where the world is shown: the camera over the whole screen.
func (w *worldScene) Viewports(screen geom.AABB) []render.Viewport {
	return render.Whole(w.arena.world.Camera(), screen)
}

func (w *worldScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	w.arena.players.Handle(events, runtime, composition)
	for _, k := range events.KeyEvents {
		if k.Action == control.ActionPress && k.Key == control.KeyP {
			composition.Show(w.arena.panel.Name())
		}
	}
}

func (w *worldScene) Focusable() bool { return true }

// panelScene is a modal box toggled by P: while shown it takes the input,
// and the world beneath keeps ticking.
type panelScene struct{ arena *gameplayArena }

var _ game.Scene = (*panelScene)(nil)

func (p *panelScene) Name() string { return "panel" }

func (p *panelScene) Layers() []render.Layer { return []render.Layer{&panelRenderer{}} }

func (p *panelScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	p.arena.players.Handle(events, runtime, composition)
	for _, k := range events.KeyEvents {
		if k.Action == control.ActionPress && k.Key == control.KeyP {
			composition.Hide(p.Name())
		}
	}
}

func (p *panelScene) Focusable() bool { return true }

type panelRenderer struct{}

func (r *panelRenderer) Init(*goke.SysInit) {}

func (r *panelRenderer) Draw(screen *render.Image) {
	const w, h = 300, 140
	x, y := float32(ScreenWidth-w)/2, float32(ScreenHeight-h)/2
	render.FillRect(screen, x, y, w, h, color.RGBA{R: 235, G: 235, B: 235, A: 255})
	render.DebugPrintAt(screen, "PANEL\n\nthe world keeps ticking behind me\nP to close", int(x)+12, int(y)+12)
}

// hudScene is a passive overlay: always on top, never focusable, so it never takes input.
type hudScene struct{ arena *gameplayArena }

var _ game.Scene = (*hudScene)(nil)

func (h *hudScene) Name() string { return "hud" }

func (h *hudScene) Layers() []render.Layer { return []render.Layer{&hudRenderer{arena: h.arena}} }

func (h *hudScene) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}

func (h *hudScene) Focusable() bool { return false }

type hudRenderer struct{ arena *gameplayArena }

func (r *hudRenderer) Init(*goke.SysInit) {}

func (r *hudRenderer) Draw(screen *render.Image) {
	active := r.arena.stage.Stack().Composition().Active()
	render.DebugPrintAt(screen, fmt.Sprintf("active scene: %s  (P: toggle panel, F5: save, K: keys)", active), 8, ScreenHeight-20)
}
