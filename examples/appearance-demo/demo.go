// Command appearance-demo shows a game changing how its entities are drawn, frame by frame, with
// the world's ready-made drawing rules (plugins/world/hooks): every walker is drawn facing the way it
// goes (Facing), red while it is angry (With, reading its Mood), a ghost as a ghost whatever it
// feels (As) and the leader with a crown on (Overlay). They bounce off one another, so they turn,
// and their arrows turn with them. R makes everyone angry for a while: an effect on the world, kept
// on each entity by a rule as its Mood — what is drawn follows, the Appearance itself is never
// touched.
package main

import (
	"image/color"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	whooks "github.com/kjkrol/gram/plugins/world/hooks"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

const (
	TPS          = 60
	ScreenWidth  = 800
	ScreenHeight = 600

	// Size is every entity's side; Walkers, Ghosts and Leaders how many of each there are.
	Size    = 16
	Walkers = 30
	Ghosts  = 6
	Leaders = 1

	// rageFor is how long R keeps everyone angry.
	rageFor = 3 * time.Second
)

// rng is where every random choice in this demo comes from; the test pins its seed.
var rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

// =========================== Game ===========================

// Demo is the appearance demo — one Stage.
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram appearance demo — R: everyone angry for a while",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// Mood is what an entity feels: the With rule draws it red while Angry. Ghost and Leader mark the
// entities As and Overlay draw otherwise.
type (
	Mood   struct{ Angry bool }
	Ghost  struct{}
	Leader struct{}
)

// heading is one of the four ways an entity goes, as Facing picks its sprite.
type heading int

const (
	east heading = iota
	west
	south
	north
)

// headingOf is the way v goes, by its larger part.
func headingOf(v world.Velocity) heading {
	d := v.Dir
	switch {
	case d.X*d.X >= d.Y*d.Y && d.X >= 0:
		return east
	case d.X*d.X >= d.Y*d.Y:
		return west
	case d.Y >= 0:
		return south
	}
	return north
}

// walker is the row every kind spawns from: where it starts and how it goes.
type walker struct {
	at  geom.Vec
	vel world.Velocity
}

type mainStage struct {
	world     *world.Plugin
	collision *collision.Plugin
	players   *players.Plugin

	walker, ghost, leader kind.Of[walker]

	// facing is the sprite of each heading, calm and angry; spook the ghost's, crown the leader's.
	facing, angrySprite [4]render.SpriteID
	spook, crown        render.SpriteID

	rage, angry effect.Effect

	stack game.Scenes
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "appearance-demo" }

func (s *mainStage) Stack() game.Scenes { return s.stack }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: Walkers + Ghosts + Leaders, MinSize: Size, MaxSize: Size},
	})

	// Two effects: rage is a state of the whole game, R puts it on the world; angry is what each
	// entity feels while it lasts, its Mood turned.
	s.rage = s.world.Effects().Define("rage", effect.Spec{effect.Lasts(rageFor)})
	s.angry = s.world.Effects().Define("angry", effect.Spec{effect.Alter(func(m *Mood) { m.Angry = true })})

	kinds := s.world.Kinds()
	spec := func(more ...comp.Comp) kind.Spec {
		return append(kind.Spec{
			comp.Load(func(w walker) world.Position { return world.Position{AABB: boxAt(w.at)} }),
			comp.Load(func(w walker) world.Velocity { return w.vel }),
			comp.Const(Mood{}),
			comp.Const(collision.Collider{}),
			comp.Const(collision.Physics{Restitution: 1}),
		}, more...)
	}
	s.walker = kind.Define[walker](kinds, "walker", spec())
	s.ghost = kind.Define[walker](kinds, "ghost", spec(comp.Const(Ghost{})))
	s.leader = kind.Define[walker](kinds, "leader", spec(comp.Const(Leader{})))
	for h := range s.facing {
		s.facing[h], s.angrySprite[h] = kinds.NewSprite(), kinds.NewSprite()
	}
	s.spook, s.crown = kinds.NewSprite(), kinds.NewSprite()

	// The whole of the drawing, in order: the sprite of the heading, red while angry, a ghost drawn
	// as a ghost after that — whatever it feels — and a crown on the leader. And the rule keeping
	// everyone angry while the world is in a rage.
	if err := s.world.Hook(
		whooks.Facing(func(v world.Velocity) render.SpriteID { return s.facing[headingOf(v)] }),
		whooks.With(func(a world.Appearance, m Mood) world.Appearance {
			if m.Angry {
				for h, calm := range s.facing {
					if a.SpriteID == calm {
						a.SpriteID = s.angrySprite[h]
					}
				}
			}
			return a
		}),
		whooks.As[Ghost](world.Appearance{SpriteID: s.spook}),
		whooks.Overlay[Leader](world.Appearance{SpriteID: s.crown}),
		rule.On("rage spreads", rule.All, func(m *rule.Moment[world.Moving]) rule.Step {
			return m.During(s.rage, m.Keep(s.angry))
		}),
	); err != nil {
		return err
	}

	s.collision = collision.NewPlugin(s.world)
	if err := ctx.Use(s.collision); err != nil {
		return err
	}

	// The player's camera and R, which puts the rage on the world.
	s.players = players.NewPlugin(s.world)
	player := s.players.Local("player")
	if err := player.Bind(s.players.Defaults()...); err != nil {
		return err
	}
	if err := player.Bind(control.Command(control.KeyPress{Key: control.KeyR}, "Make everyone angry for a while",
		func(control.Context) (world.Apply, bool) { return world.Apply{Effect: s.rage}, true })); err != nil {
		return err
	}
	if err := ctx.Use(s.players); err != nil {
		return err
	}

	main := &mainScene{stage: s}
	stack, err := game.NewStack(main)
	if err != nil {
		return err
	}
	s.stack = stack
	comp := stack.Composition()
	comp.Show(main.Name())
	return ctx.Track(comp)
}

// boxAt is the box of side Size round at.
func boxAt(at geom.Vec) plane.AABB {
	return plane.NewAABB(geom.NewVec(at.X-Size/2, at.Y-Size/2), Size, Size)
}

// Restore loads nothing: the demo starts afresh every time.
func (s *mainStage) Restore(game.Persistence) (bool, error) { return false, nil }

// Spawn puts the walkers, the ghosts and the leader on a grid, each going one of the four ways.
func (s *mainStage) Spawn() error {
	total := Walkers + Ghosts + Leaders
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, Size)
	ways := [4]geom.Vec{geom.NewVec(1, 0), geom.NewVec(-1, 0), geom.NewVec(0, 1), geom.NewVec(0, -1)}
	entries := make([]kind.Entry, total)
	for i := range entries {
		w := walker{at: placement.Place(i, total).Center(), vel: world.Velocity{Dir: ways[rng.IntN(4)], Value: 40 + 40*rng.Float64()}}
		switch {
		case i < Leaders:
			entries[i] = s.leader.Entry(w)
		case i < Leaders+Ghosts:
			entries[i] = s.ghost.Entry(w)
		default:
			entries[i] = s.walker.Entry(w)
		}
	}
	s.world.Seed(entries...)
	return nil
}

func (s *mainStage) Update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct{ stage *mainStage }

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage
	s.world.WithRenderer(s.atlas())
	return []render.Layer{
		render.NewCachedRenderer(render.SolidBackground{Color: color.RGBA{R: 40, G: 44, B: 52, A: 255}}, ScreenWidth, ScreenHeight),
		render.NewComposer(s.world.Renderer()),
	}
}

// atlas draws every sprite: a square with a light nose on the side it faces, blue when calm, red
// when angry; a pale ghost; a gold crown along the top.
func (s *mainStage) atlas() *render.Atlas {
	calm, angry := color.RGBA{R: 70, G: 130, B: 220, A: 255}, color.RGBA{R: 220, G: 60, B: 50, A: 255}
	nose := color.RGBA{R: 245, G: 245, B: 230, A: 255}
	atlas := render.NewAtlas()
	for _, sprite := range []render.SpriteID{s.walker.SpriteID(), s.ghost.SpriteID(), s.leader.SpriteID()} {
		atlas.RegisterAt(sprite, Size, render.Solid(calm))
	}
	for h := range s.facing {
		atlas.RegisterAt(s.facing[h], Size, facingSprite(heading(h), calm, nose))
		atlas.RegisterAt(s.angrySprite[h], Size, facingSprite(heading(h), angry, nose))
	}
	atlas.RegisterAt(s.spook, Size, func(dst *render.Canvas, size int) {
		r := float32(size) / 2
		dst.FillCircle(r, r, r-1, color.RGBA{R: 225, G: 230, B: 245, A: 200})
		dst.FillRect(r-4, r-3, 2, 3, color.RGBA{A: 255})
		dst.FillRect(r+2, r-3, 2, 3, color.RGBA{A: 255})
	})
	atlas.RegisterAt(s.crown, Size, func(dst *render.Canvas, size int) {
		gold := color.RGBA{R: 240, G: 200, B: 40, A: 255}
		dst.FillRect(1, 0, float32(size)-2, 4, gold)
		for x := float32(1); x < float32(size)-2; x += 5 {
			dst.FillRect(x, 0, 2, 6, gold)
		}
	})
	atlas.Close()
	return atlas
}

// facingSprite is a square of body with a nose on the side h faces.
func facingSprite(h heading, body, nose color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		s := float32(size)
		dst.FillRect(0, 0, s, s, body)
		switch h {
		case east:
			dst.FillRect(s-4, s/2-2, 4, 4, nose)
		case west:
			dst.FillRect(0, s/2-2, 4, 4, nose)
		case south:
			dst.FillRect(s/2-2, s-4, 4, 4, nose)
		case north:
			dst.FillRect(s/2-2, 0, 4, 4, nose)
		}
	}
}

// Viewports are where the world is shown: the local player's view.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, _ game.Composition) {
	m.stage.players.EventHandler().HandleEvents(events)
	for _, k := range events.KeyEvents {
		if k.Action == control.ActionPress && k.Key == control.KeyEscape {
			runtime.Quit()
		}
	}
}

func (m *mainScene) Focusable() bool { return true }
