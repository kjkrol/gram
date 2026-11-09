package world_test

import (
	"cmp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

const (
	bridgeKind = "bridge"
	deckKind   = "deck"
	// a deck's side; a bridge's handle is three decks long, longer than an entity may be
	deckSide = 10
)

// where a deck or a bridge begins along x
type deckRow struct{ x float32 }

// wholeStage is a world of bridges, each a handle over three decks, one seeded where seed says;
// at each tick its system hands Write.Despawn the entities in gone
type wholeStage struct {
	seed bool
	load bool

	world   *world.Plugin
	ecs     *goke.ECS
	bridge  kind.Of[deckRow]
	deck    kind.Of[deckRow]
	gone    []uid.UID64
	goes    goke.Runnable
	base    goke.Comp[world.Base]
	whole   goke.OptComp[world.Whole]
	pieceOf goke.OptComp[world.PieceOf]
	query   *goke.Query
	stack   game.Scenes
}

func wholesConfig() world.Config {
	cfg := testWorldConfig()
	cfg.Entities = world.EntitiesCfg{MaxCount: 8, MinSize: 5, MaxSize: deckSide}
	return cfg
}

// defineBridges defines the kinds of a bridge's handle, carrying Whole, and of its decks
func defineBridges(w *world.Plugin) (bridge, deck kind.Of[deckRow]) {
	for _, k := range []struct {
		name string
		long float32
		more []comp.Comp
	}{
		{bridgeKind, 3 * deckSide, []comp.Comp{comp.Const(world.Whole{})}},
		{deckKind, deckSide, nil},
	} {
		kind.Define[deckRow](w.Kinds(), k.name, append(kind.Spec{
			comp.Load(func(r deckRow) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x, 100), k.long, deckSide)}
			}),
			comp.Const(world.Velocity{}),
			comp.Const(world.Fixed{}),
		}, k.more...))
	}
	return kind.Named[deckRow](w.Kinds(), bridgeKind), kind.Named[deckRow](w.Kinds(), deckKind)
}

func (g *wholeStage) Name() string { return "stage" }

func (g *wholeStage) Init(ctx game.Initializer) error {
	g.world, g.ecs = ctx.UseWorld(wholesConfig()), ctx.ECS()
	ctx.Setup(g)
	g.bridge, g.deck = defineBridges(g.world)
	return nil
}

func (g *wholeStage) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		g.query = si.NewQueryBuilder(&g.base).Optional(&g.whole).Optional(&g.pieceOf).Build()
	}}}
}

func (g *wholeStage) Loaded() error { return nil }

func (g *wholeStage) Spawn() error {
	if g.seed {
		g.world.Write().SeedWhole(g.bridged(100))
	}
	return nil
}

// bridged is the handle of a bridge from x and its three decks
func (g *wholeStage) bridged(x float32) (kind.Entry, kind.Entry, kind.Entry, kind.Entry) {
	return g.bridge.Entry(deckRow{x}), g.deck.Entry(deckRow{x}),
		g.deck.Entry(deckRow{x + deckSide}), g.deck.Entry(deckRow{x + 2*deckSide})
}

func (g *wholeStage) Update(ctx goke.RunCtx, d time.Duration) {
	if g.goes != nil {
		ctx.Run(g.goes, d)
	}
	g.world.RunPlan(ctx, d)
}

func (g *wholeStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

func (g *wholeStage) start(t *testing.T) *engine.Engine {
	t.Helper()
	eng := engine.NewEngine(oneStageGame{stage: g})
	if g.load {
		if err := eng.Load("", ""); err != nil {
			t.Fatalf("Load: %v", err)
		}
	}
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	g.goes = g.ecs.RegSys(goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		for _, id := range g.gone {
			g.world.Write().Despawn(cb, id)
		}
		g.gone = g.gone[:0]
	}})
	return eng
}

func (g *wholeStage) tick() { g.ecs.Tick(time.Second / 60) }

// what stands: where it begins along x, whether it is a handle, the handle it is a piece of
type standing struct {
	x      float32
	handle bool
	of     uid.UID64
}

func (g *wholeStage) standing() map[uid.UID64]standing {
	got := map[uid.UID64]standing{}
	for g.query.All(); g.query.Next(); {
		cur := g.query.Cursor()
		bases := g.base.Slice(cur)
		var of []world.PieceOf
		if g.pieceOf.Present(cur) {
			of = g.pieceOf.Slice(cur)
		}
		for i, id := range cur.IDs {
			s := standing{x: bases[i].Pos.TopLeft.X, handle: g.whole.Present(cur)}
			if of != nil {
				s.of = of[i].Whole
			}
			got[id] = s
		}
	}
	return got
}

// bridges is the handles standing, each with its pieces in the order they lie along x, failing
// where a thing is neither a handle nor a piece of one standing
func (g *wholeStage) bridges(t *testing.T) map[uid.UID64][]uid.UID64 {
	t.Helper()
	all := g.standing()
	got := map[uid.UID64][]uid.UID64{}
	for id, s := range all {
		if s.handle {
			if s.of != 0 {
				t.Errorf("the handle %v is a piece of %v", id, s.of)
			}
			got[id] = nil
		}
	}
	for id, s := range all {
		switch _, ok := got[s.of]; {
		case s.handle:
		case !ok:
			t.Errorf("a deck at %v is a piece of %v, no handle standing", s.x, s.of)
		default:
			got[s.of] = append(got[s.of], id)
		}
	}
	for _, pieces := range got {
		slices.SortFunc(pieces, func(a, b uid.UID64) int { return cmp.Compare(all[a].x, all[b].x) })
	}
	return got
}

// oneBridge is the one bridge standing, failing unless it is a handle over three decks
func (g *wholeStage) oneBridge(t *testing.T) (uid.UID64, []uid.UID64) {
	t.Helper()
	bridges := g.bridges(t)
	if len(bridges) != 1 {
		t.Fatalf("%d bridges stand, want 1: %v", len(bridges), g.standing())
	}
	for handle, decks := range bridges {
		if len(decks) != 3 {
			t.Fatalf("the bridge stands over %d decks, want 3: %v", len(decks), g.standing())
		}
		return handle, decks
	}
	return 0, nil
}

func TestWhole_SpawnedInOneStepItsPiecesUnderItsHandle(t *testing.T) {
	g := &wholeStage{}
	g.start(t)
	g.world.Write().SpawnWhole(g.bridged(100))
	g.tick()
	g.oneBridge(t)
	if n := g.world.Res.Telemetry.Count; n != 4 {
		t.Errorf("telemetry counts %d, want the handle and its 3 decks", n)
	}
}

func TestWhole_SeededItsPiecesUnderItsHandleFromTheStart(t *testing.T) {
	g := &wholeStage{seed: true}
	g.start(t)
	g.oneBridge(t)
	g.tick()
	g.oneBridge(t)
}

func TestWhole_ASaveAndALoadKeepTheHandleAndItsPieces(t *testing.T) {
	t.Chdir(t.TempDir())
	was := &wholeStage{seed: true}
	eng := was.start(t)
	handle, decks := was.oneBridge(t)
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := &wholeStage{load: true}
	now.start(t)
	if h, d := now.oneBridge(t); h != handle || !slices.Equal(d, decks) {
		t.Errorf("loaded the handle %v over %v, want %v over %v as saved", h, d, handle, decks)
	}
	now.gone = append(now.gone, handle)
	now.tick()
	if left := now.standing(); len(left) != 0 {
		t.Errorf("after the loaded handle's despawn %v stand, want nothing", left)
	}
}

func TestWhole_ItsHandleGoneItsPiecesGoByTheTicksEnd(t *testing.T) {
	for _, by := range []string{"Write.Despawn", "its own Despawn"} {
		t.Run(by, func(t *testing.T) {
			g := &wholeStage{seed: true}
			g.start(t)
			handle, _ := g.oneBridge(t)
			if by == "Write.Despawn" {
				g.gone = append(g.gone, handle)
			} else {
				g.world.Write().Carrier().PutFrom(handle, world.Despawn{})
			}
			g.tick()
			if left := g.standing(); len(left) != 0 {
				t.Errorf("a tick after the handle's despawn %v stand, want nothing", left)
			}
			if n := g.world.Res.Telemetry.Count; n != 0 {
				t.Errorf("telemetry counts %d, want none", n)
			}
		})
	}
}

func TestWhole_APieceGoneLeavesItsHandleAndTheOthers(t *testing.T) {
	g := &wholeStage{seed: true}
	g.start(t)
	handle, decks := g.oneBridge(t)
	g.world.Write().Carrier().PutFrom(decks[1], world.Despawn{})
	g.tick()
	bridges := g.bridges(t)
	if left := bridges[handle]; len(bridges) != 1 || !slices.Equal(left, []uid.UID64{decks[0],
		decks[2]}) {
		t.Errorf("after the middle deck's despawn the bridges %v stand, want %v over %v", bridges,
			handle, []uid.UID64{decks[0], decks[2]})
	}
}

func TestWhole_OneRefusedSpawnsNoneOfIt(t *testing.T) {
	g := &wholeStage{seed: true}
	g.start(t)
	write := g.world.Write()
	_, deck, deck2, deck3 := g.bridged(200)
	// a handle whose kind carries no Whole
	write.SpawnWhole(deck, deck2, deck3)
	// a piece longer than an entity may be
	write.SpawnWhole(g.bridge.Entry(deckRow{200}), deck, g.bridge.Entry(deckRow{210}))
	// the first of two wholes the world has room for alone
	write.SpawnWhole(g.bridged(300))
	write.SpawnWhole(g.bridged(400))
	g.tick()
	bridges := g.bridges(t)
	all := g.standing()
	var at []float32
	for handle, decks := range bridges {
		if len(decks) != 3 {
			t.Errorf("a bridge at %v stands over %d decks, want 3", all[handle].x, len(decks))
		}
		at = append(at, all[handle].x)
	}
	slices.Sort(at)
	if !slices.Equal(at, []float32{100, 300}) || len(all) != 8 {
		t.Errorf("bridges at %v, %d things, want the seeded and the first with room, 8", at,
			len(all))
	}
}

func TestWhole_SeedingAHandleWithoutWholeIsRefused(t *testing.T) {
	w := world.NewPlugin(wholesConfig())
	_, deck := defineBridges(w)
	w.Write().SeedWhole(deck.Entry(deckRow{100}), deck.Entry(deckRow{100}))
	if err := w.Populate(); err == nil || !strings.Contains(err.Error(), "carries no Whole") {
		t.Errorf("seeding a deck as a handle: %v, want it refused for carrying no Whole", err)
	}
}
