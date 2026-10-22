package engine

import (
	"fmt"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type stageRuntime struct {
	host         *ecsHost
	stage        game.Stage
	world        *world.Plugin
	carrier      *command.Carrier
	sceneDrawers map[string][]render.Drawer
}

func (e *Engine) enterStage(stage game.Stage, load *string) (*stageRuntime, error) {
	host := newECSHost()

	ctx := &initializer{host: host, tps: e.tps, pool: e.pool,
		handlers: []plugin.CommandHandler{&e.commands}}
	if err := stage.Init(ctx); err != nil {
		return nil, err
	}
	if err := ctx.deliver(); err != nil {
		return nil, err
	}
	ctx.look()
	carrier, err := ctx.carrier()
	if err != nil {
		return nil, err
	}

	if load != nil {
		if err := (&persistence{host: host}).Load(e.savePath(stage.Name()), *load); err != nil {
			return nil, err
		}
		if err := stage.Loaded(); err != nil {
			return nil, err
		}
	} else {
		if err := stage.Spawn(); err != nil {
			return nil, err
		}
		if err := host.runPopulate(); err != nil {
			return nil, err
		}
	}

	// the clock replays what simulates as many times as its tempo says, none in the pause, and
	// settles what reads goke's memory after every Sync of the Stage's own Update too
	if ctx.world != nil {
		clk := ctx.world.Write().Clock()
		host.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
			stage.Update(clk.Settling(rc), d)
			clk.Replay(rc, d)
		})
	} else {
		host.ecs.SetPlan(stage.Update)
	}

	sceneDrawers := make(map[string][]render.Drawer)
	for _, sc := range stage.Stack().All() {
		drawers := sc.Drawers()
		if err := checkDrawers(sc, drawers); err != nil {
			return nil, err
		}
		for _, l := range drawers {
			host.registerDrawer(l)
		}
		sceneDrawers[sc.Name()] = drawers
	}

	host.flushPendingSetup()

	return &stageRuntime{
		host:         host,
		stage:        stage,
		world:        ctx.world,
		carrier:      carrier,
		sceneDrawers: sceneDrawers,
	}, nil
}

func checkDrawers(sc game.Scene, drawers []render.Drawer) error {
	for _, l := range drawers {
		if _, ok := l.(render.Renderer); ok {
			continue
		}
		if _, ok := l.(render.Picture); ok {
			return fmt.Errorf("gram: scene %q: %T is a world picture; show it through a "+
				"ui.Image(render.NewFeed(camera, it))", sc.Name(), l)
		}
		if _, ok := l.(render.Source); ok {
			return fmt.Errorf("gram: scene %q: %T is a render.Source; its plugin draws it "+
				"(ctx.Draws), a ui.Image(render.NewFeed(camera, world.Picture())) shows it",
				sc.Name(), l)
		}
		return fmt.Errorf("gram: scene %q: %T is no Renderer", sc.Name(), l)
	}
	return nil
}

func (r *stageRuntime) drawScene(screen *render.Image, sc game.Scene) {
	for _, l := range r.sceneDrawers[sc.Name()] {
		l.(render.Renderer).Draw(screen)
	}
}
