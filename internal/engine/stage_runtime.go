package engine

import (
	"fmt"
	"reflect"
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
	publishers   []plugin.Publisher
	prepared     bool // its first frame readied (Engine.prepare)
	firsts       int  // its first frames logged
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

	for _, s := range ctx.draws {
		if b, ok := s.(plugin.Bound); ok {
			host.registerBound(b)
		}
	}
	sceneDrawers := make(map[string][]render.Drawer)
	var drawn []any // the scenes' drawers in their order
	for _, sc := range stage.Stack().All() {
		drawers := sc.Drawers()
		if err := checkDrawers(sc, drawers); err != nil {
			return nil, err
		}
		for _, l := range drawers {
			host.registerBound(l)
			drawn = append(drawn, l)
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
		publishers:   publishersOf(host.tracked, ctx.draws, drawn),
	}, nil
}

// publishersOf are the Publishers among the tracked values, the sources drawn and the scenes'
// drawers, each once, in that order
func publishersOf(tracked []any, sources []render.Source, drawers []any) []plugin.Publisher {
	all := append([]any(nil), tracked...)
	for _, s := range sources {
		all = append(all, s)
	}
	all = append(all, drawers...)
	var out []plugin.Publisher
	seen := map[any]bool{}
	for _, v := range all {
		p, ok := v.(plugin.Publisher)
		if !ok {
			continue
		}
		if reflect.TypeOf(v).Comparable() {
			if seen[v] {
				continue
			}
			seen[v] = true
		}
		out = append(out, p)
	}
	return out
}

// publish has every Publisher publish, then swap
func (r *stageRuntime) publish() {
	for _, p := range r.publishers {
		p.Publish()
	}
	for _, p := range r.publishers {
		p.Swap()
	}
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
