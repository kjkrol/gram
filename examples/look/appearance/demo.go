package main

import (
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	ScreenWidth  = 800
	ScreenHeight = 600

	Size    = 16
	Walkers = 30
	Ghosts  = 6
	Leaders = 1

	rageFor = 3 * time.Second
)

type arena struct {
	world     *world.Plugin
	collision *collision.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player
}

func main() {
	props := game.Props{
		Title: "gram appearance demo",
		Description: "Walkers drawn the way they go, stamping while angry; R: everyone angry " +
			"for a while.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(AppearanceStage).
		World(world.Config{
			Space: world.SpaceCfg{
				Width:  ScreenWidth,
				Height: ScreenHeight,
				Edges:  aabbworld.Torus,
			},
			Entities: world.EntitiesCfg{
				MaxCount: Walkers + Ghosts + Leaders,
				MinSize:  Size / 2,
				MaxSize:  Size,
			},
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRules).
		Commands(a.defineCommands).
		Controls(a.bindKeys).
		Spawn(a.spawnUnits).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.collision = collision.NewPlugin(a.world)
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision, a.cameras)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineEffects() {
	a.world.Effects().Define(RageEf, effect.Spec{effect.Lasts(rageFor)})
	a.world.Effects().Define(AngryEf, effect.Spec{})
}

func (a *arena) defineRules() {
	a.world.Roles().Define(MoodyRole, act.Branch[world.Moving]{
		Name: "rage spreads",
		When: cond.On(a.world).If(a.world.Effects().Named(RageEf)),
		Then: act.Self.Hold(a.world.Effects().Named(AngryEf)),
	})
}

func (a *arena) defineCommands() {
	a.world.Commands().Define(RageCmd,
		ceremony.Put(a.world.Effects().Named(RageEf)).On(ceremony.World))
}

type walker struct {
	at   geom.Vec
	w, h float64
	vel  world.Velocity
}

// what every walker carries: its box and its way from its row, a body that bounces, the mood
func walkerSpec(w *world.Plugin) kind.Spec {
	return kind.Spec{
		comp.Load(func(wk walker) world.Position {
			top := geom.NewVec(wk.at.X-wk.w/2, wk.at.Y-wk.h/2)
			return world.Position{AABB: plane.NewAABB(top, wk.w, wk.h)}
		}),
		comp.Load(func(wk walker) world.Velocity { return wk.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
		ceremony.Plays(w.Roles().Named(MoodyRole)),
	}
}

func (a *arena) defineFigures() { a.world.Figures(newHero(), leader{}, ghost{}) }

func (a *arena) bindKeys() error {
	return a.player.Bind(
		control.Give(control.KeyPress{Key: control.KeyR}, "Make everyone angry for a while",
			a.world.Commands().Named(RageCmd)),
	)
}

func (a *arena) spawnUnits() {
	total := Walkers + Ghosts + Leaders
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, Size)
	ways := [4]geom.Vec{
		geom.NewVec(1, 0),
		geom.NewVec(-1, 0),
		geom.NewVec(0, 1),
		geom.NewVec(0, -1),
	}
	leaderKind := kind.Named[walker](a.world.Kinds(), LeaderKind)
	ghostKind := kind.Named[walker](a.world.Kinds(), GhostKind)
	walkerKind := kind.Named[walker](a.world.Kinds(), WalkerKind)
	entries := make([]kind.Entry, total)
	for i := range entries {
		w := walker{
			at: placement.Place(i, total).Center(),
			w:  Size, h: Size,
			vel: world.Velocity{Dir: ways[rand.IntN(4)], Value: 40 + 40*rand.Float64()},
		}
		switch {
		case i < Leaders:
			entries[i] = leaderKind.Entry(w)
		case i < Leaders+Ghosts:
			entries[i] = ghostKind.Entry(w)
		default:
			w.w, w.h = Size/2, Size // the figure's box, twice as tall as wide
			entries[i] = walkerKind.Entry(w)
		}
	}
	a.world.Seed(entries...)
}

func (a *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
