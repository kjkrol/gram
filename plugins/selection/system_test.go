package selection

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type pendingSeed struct {
	x, y, size float64
	alt        float64
	id         *uid.UID64
	plain      bool
}

// harness seeds entities, drives the system through a player's bindings and a tick, and reads
// Selected back; seed only queues, start performs the single Setup.
type harness struct {
	t         *testing.T
	space     *aabbworld.Space
	players   *players.Plugin
	local     *players.Player
	sel       *Plugin
	sys       *SelectionSystem
	handler   control.EventHandler
	ecs       *goke.ECS
	pos       goke.Comp[world.Base]
	z         goke.Comp[world.Z]
	tag       goke.Comp[plugin.Tags[Family]]
	marks     goke.Comp[plugin.Tags[Family]]
	tags      Tags
	selectedQ *goke.Query
	handle    goke.Runnable
	pending   []pendingSeed
	items     []aabbworld.Item
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessIn(t, world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
}

// newHarnessIn is newHarness over a world of the given configuration — an isometric one, say.
func newHarnessIn(t *testing.T, cfg world.Config) *harness {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{
		Width: 1000, Height: 1000,
		BucketSize: 64,
	})
	if err != nil {
		t.Fatalf("aabbworld.NewSpace: %v", err)
	}

	w := world.NewPlugin(cfg)
	sel := NewPlugin(w)
	pl := players.NewPlugin(w, sel)
	local := pl.Local("tester")
	if err := local.Bind(sel.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	tags := Tags{Selectable: 0, Selected: 1}
	sys := NewSelectionSystem(&sel.selects, space, w.Camera(), tags)

	return &harness{t: t, space: space, players: pl, local: local, sel: sel, sys: sys, handler: pl.EventHandler(), ecs: goke.New(), tags: tags}
}

// seed queues a Selectable size x size entity at (x,y); the returned id is filled in by start.
func (h *harness) seed(x, y, size float64) *uid.UID64 {
	id := new(uid.UID64)
	h.pending = append(h.pending, pendingSeed{x: x, y: y, size: size, id: id})
	return id
}

// seedPlain queues an entity nobody may select — terrain, say.
func (h *harness) seedPlain(x, y, size float64) *uid.UID64 {
	id := h.seed(x, y, size)
	h.pending[len(h.pending)-1].plain = true
	return id
}

// start builds the query and spawns every queued seed — call once, after all seed() calls.
func (h *harness) start() {
	h.t.Helper()
	h.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		h.selectedQ = si.NewQueryBuilder(&h.marks).Build()
		if len(h.pending) == 0 {
			return
		}
		factories := map[bool]*goke.Factory{false: si.NewFactory(&h.pos, &h.tag, &h.z), true: si.NewFactory(&h.pos)}
		for plain, f := range factories {
			var seeds []pendingSeed
			for _, spec := range h.pending {
				if spec.plain == plain {
					seeds = append(seeds, spec)
				}
			}
			f.Create(len(seeds))
			i := 0
			for f.Next() {
				positions := h.pos.Slice(&f.Cursor)
				for j, id := range f.Cursor.IDs {
					spec := seeds[i]
					*spec.id = id
					aabb := plane.NewAABB(geom.NewVec(spec.x, spec.y), spec.size, spec.size)
					positions[j].Pos = world.Position{AABB: aabb}
					if !plain {
						h.tag.Slice(&f.Cursor)[j] = plugin.Tags[Family](0).With(h.tags.Selectable)
						h.z.Slice(&f.Cursor)[j] = world.Z{Altitude: spec.alt}
					}
					h.items = append(h.items, aabbworld.Item{ID: id, Box: aabb})
					i++
				}
			}
		}
		h.space.Rebuild(h.items)
	}})

	h.handle = h.ecs.RegSys(h.sys)
	h.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(h.handle, d)
		ctx.Sync()
	})
}

func (h *harness) click(x, y int, shift bool) {
	events := &control.InputEvents{}
	events.Modifiers.Shift = shift
	events.AddClickEvent(x, y, ebiten.MouseButtonLeft, control.ActionPress)
	events.AddClickEvent(x, y, ebiten.MouseButtonLeft, control.ActionRelease)
	h.handler.HandleEvents(events)
	h.ecs.Tick(time.Second)
}

func (h *harness) drag(x0, y0, x1, y1 int, shift bool) {
	events := &control.InputEvents{}
	events.Modifiers.Shift = shift
	events.AddClickEvent(x0, y0, ebiten.MouseButtonLeft, control.ActionPress)
	events.AddClickEvent(x1, y1, ebiten.MouseButtonLeft, control.ActionRelease)
	h.handler.HandleEvents(events)
	h.ecs.Tick(time.Second)
}

func (h *harness) isSelected(id uid.UID64) bool {
	h.t.Helper()
	h.selectedQ.All()
	for h.selectedQ.Next() {
		cur := h.selectedQ.Cursor()
		marks := h.marks.Slice(cur)
		for i, got := range cur.IDs {
			if got == id {
				return marks[i].Has(h.tags.Selected)
			}
		}
	}
	return false
}

// seedHigh queues a Selectable entity standing alt above the ground.
func (h *harness) seedHigh(x, y, size, alt float64) *uid.UID64 {
	id := h.seed(x, y, size)
	h.pending[len(h.pending)-1].alt = alt
	return id
}

func TestSystem_Update_ClickPicksWhereTheEntityIsDrawnThroughAnIsometricCamera(t *testing.T) {
	h := newHarnessIn(t, world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Camera:   camera.Config{ViewportWidth: 800, ViewportHeight: 600, Projection: camera.Isometric{Cell: 32, HeightUnit: 1}},
		Quasi3D:  true,
	})
	hawk := h.seedHigh(500, 500, 10, 40)
	walker := h.seed(560, 560, 10)
	h.start()
	cam := h.local.Camera
	cam.MoveTo(400, 400)
	cam.Pan(-400, -300)

	// The hawk is drawn as a billboard 40 up over its centre; a click on it selects it.
	sx, sy := cam.Project(505, 505, 40)
	h.click(int(sx), int(sy)-5, false)
	if !h.isSelected(*hawk) || h.isSelected(*walker) {
		t.Errorf("clicking the hawk where it is drawn: hawk %v, walker %v; want the hawk alone", h.isSelected(*hawk), h.isSelected(*walker))
	}
	// A click on the ground under the hawk's footprint hits nothing.
	gx, gy := cam.Project(505, 505, 0)
	h.click(int(gx), int(gy), false)
	if h.isSelected(*hawk) {
		t.Error("clicking the ground under the hawk selected it")
	}
	// The walker on the ground is where its footprint is drawn.
	wx, wy := cam.Project(565, 565, 0)
	h.click(int(wx), int(wy)-3, false)
	if !h.isSelected(*walker) {
		t.Error("clicking the walker where it stands did not select it")
	}
}

func TestSystem_Update_ClickSelectsHitEntity(t *testing.T) {
	h := newHarness(t)
	id := h.seed(50, 50, 10)
	h.start()

	h.click(55, 55, false)

	if !h.isSelected(*id) {
		t.Error("expected the entity under the click to be Selected")
	}
}

func TestSystem_Update_ClickOnEmptySpaceClearsSelection(t *testing.T) {
	h := newHarness(t)
	id := h.seed(50, 50, 10)
	h.start()

	h.click(55, 55, false)
	if !h.isSelected(*id) {
		t.Fatal("sanity check failed: expected entity to be selected after first click")
	}

	h.click(500, 500, false)

	if h.isSelected(*id) {
		t.Error("expected a non-additive click on empty space to clear the previous selection")
	}
}

func TestSystem_Update_DragSelectsEntitiesInsideBox_ReplacesOutside(t *testing.T) {
	h := newHarness(t)
	inside1 := h.seed(20, 20, 10)
	inside2 := h.seed(80, 80, 10)
	outside := h.seed(500, 500, 10)
	h.start()

	h.click(505, 505, false)
	if !h.isSelected(*outside) {
		t.Fatal("sanity check failed: expected outside entity to be selected first")
	}

	h.drag(0, 0, 100, 100, false)

	if !h.isSelected(*inside1) || !h.isSelected(*inside2) {
		t.Errorf("expected both entities inside the drag box to be Selected")
	}
	if h.isSelected(*outside) {
		t.Error("expected the entity outside the drag box to lose Selected (non-additive drag replaces)")
	}
}

func TestSystem_Update_ShiftClickAddsToExistingSelection(t *testing.T) {
	h := newHarness(t)
	first := h.seed(20, 20, 10)
	second := h.seed(200, 200, 10)
	h.start()

	h.click(25, 25, false)
	if !h.isSelected(*first) {
		t.Fatal("sanity check failed: expected first entity to be selected")
	}

	h.click(205, 205, true)

	if !h.isSelected(*first) {
		t.Error("expected the first selection to survive a Shift-click elsewhere")
	}
	if !h.isSelected(*second) {
		t.Error("expected the Shift-clicked entity to also be Selected")
	}
}

func TestSystem_Update_DragAcrossMultipleTicks(t *testing.T) {
	h := newHarness(t)
	id := h.seed(50, 50, 10)
	h.start()

	press := &control.InputEvents{}
	press.AddClickEvent(40, 40, ebiten.MouseButtonLeft, control.ActionPress)
	h.handler.HandleEvents(press)
	h.ecs.Tick(time.Second)

	if h.isSelected(*id) {
		t.Fatal("sanity check failed: press alone (no release yet) should not select anything")
	}

	release := &control.InputEvents{}
	release.AddClickEvent(60, 60, ebiten.MouseButtonLeft, control.ActionRelease)
	h.handler.HandleEvents(release)
	h.ecs.Tick(time.Second)

	if !h.isSelected(*id) {
		t.Error("expected the entity inside the drag box to be Selected after release, even though press/release arrived in separate HandleEvents calls")
	}
}

func TestSystem_Update_SelectByID_TagsExactlyGivenEntities(t *testing.T) {
	h := newHarness(t)
	target := h.seed(20, 20, 10)
	other := h.seed(200, 200, 10)
	h.start()

	h.click(205, 205, false)
	if !h.isSelected(*other) {
		t.Fatal("sanity check failed: expected other to be selected first")
	}

	h.sel.selects.Add(h.local.ID, Select{IDs: []uid.UID64{*target}})
	h.ecs.Tick(time.Second)

	if !h.isSelected(*target) {
		t.Error("expected Select to tag the given entity as Selected")
	}
	if h.isSelected(*other) {
		t.Error("expected Select to replace the previous selection, not add to it")
	}
}

func TestSystem_DragBox_TracksLiveDragState(t *testing.T) {
	h := newHarness(t)
	h.start()

	if _, _, dragging := h.local.DragBox(); dragging {
		t.Fatal("sanity check failed: expected no drag in progress before any input")
	}

	press := &control.InputEvents{}
	press.AddClickEvent(10, 10, ebiten.MouseButtonLeft, control.ActionPress)
	h.handler.HandleEvents(press)

	start, current, dragging := h.local.DragBox()
	if !dragging {
		t.Fatal("expected dragging=true right after a press")
	}
	if start != geom.NewVec(10, 10) || current != geom.NewVec(10, 10) {
		t.Errorf("start/current = %v/%v, want (10,10)/(10,10)", start, current)
	}

	move := &control.InputEvents{MousePos: geom.NewVec(40, 60)}
	h.handler.HandleEvents(move)

	start, current, dragging = h.local.DragBox()
	if !dragging {
		t.Error("expected dragging to remain true while the button is still held")
	}
	if start != geom.NewVec(10, 10) {
		t.Errorf("start = %v, want unchanged (10,10)", start)
	}
	if current != geom.NewVec(40, 60) {
		t.Errorf("current = %v, want (40,60) (updated from MousePos with no click event)", current)
	}

	release := &control.InputEvents{}
	release.AddClickEvent(40, 60, ebiten.MouseButtonLeft, control.ActionRelease)
	h.handler.HandleEvents(release)

	if _, _, dragging := h.local.DragBox(); dragging {
		t.Error("expected dragging=false after release")
	}
}

func TestSelection_PassesByWhatIsNotSelectable(t *testing.T) {
	h := newHarness(t)
	unit := h.seed(100, 100, 10)
	terrain := h.seedPlain(130, 100, 10)
	h.start()

	h.drag(90, 90, 150, 120, false)
	if !h.isSelected(*unit) {
		t.Error("expected the Selectable entity in the drag box to be Selected")
	}
	if h.isSelected(*terrain) {
		t.Error("expected the entity without Selectable to be passed by")
	}
}
