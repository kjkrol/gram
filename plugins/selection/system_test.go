package selection

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

type pendingSeed struct {
	x, y, size float64
	alt        float64
	id         *uid.UID64
	plain      bool
	owner      control.PlayerID // who owns it; nobody for Nobody
}

// harness seeds entities, drives the system through a player's bindings and a tick, and reads
// Selected back; seed only queues, start performs the single Setup.
type harness struct {
	t         *testing.T
	space     *aabbworld.Space
	players   *players.Plugin
	local     *players.Player
	sel       *Plugin
	world     *world.Plugin
	sys       *SelectionSystem
	handler   control.EventHandler
	ecs       *goke.ECS
	pos       goke.Comp[world.Base]
	z         goke.Comp[world.Z]
	tag       goke.Comp[tag.Tags[Family]]
	owners    goke.Comp[tag.Tags[owner.Family]]
	marks     goke.Comp[tag.Tags[Family]]
	tags      Tags
	selectedQ *goke.Query
	handle    goke.Runnable
	moveQ     *goke.Query
	moveBase  goke.Comp[world.Base]
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
	return newHarnessViewed(t, cfg, nil)
}

// newHarnessViewed is newHarnessIn with view applied to the world before anyone asks it for a camera.
func newHarnessViewed(t *testing.T, cfg world.Config, view func(*world.Plugin)) *harness {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{
		Width: 1000, Height: 1000,
		BucketSize: 64,
	})
	if err != nil {
		t.Fatalf("aabbworld.NewSpace: %v", err)
	}

	w := world.NewPlugin(cfg)
	if view != nil {
		view(w)
	}
	sel := NewPlugin(w)
	pl := players.NewPlugin(w, sel)
	local := pl.Local("tester")
	if err := local.Bind(sel.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	tags := Tags{Selectable: 0, Selected: 1}
	sys := NewSelectionSystem(&sel.selects, space, tags, w.Look)
	sys.marqueeQueue, sys.marquees = &sel.marqueeQueue, &sel.marquees

	return &harness{t: t, world: w, space: space, players: pl, local: local, sel: sel, sys: sys, handler: pl.EventHandler(), ecs: goke.New(), tags: tags}
}

// seed queues a Selectable size x size entity at (x,y), the local player's; the returned id is
// filled in by start.
func (h *harness) seed(x, y, size float64) *uid.UID64 {
	return h.seedOwned(x, y, size, h.local.ID)
}

// seedOwned is seed of an entity by owns; control.Nobody for one nobody owns.
func (h *harness) seedOwned(x, y, size float64, by control.PlayerID) *uid.UID64 {
	id := new(uid.UID64)
	h.pending = append(h.pending, pendingSeed{x: x, y: y, size: size, id: id, owner: by})
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
		h.moveQ = si.NewQueryBuilder(&h.moveBase).Build()
		if len(h.pending) == 0 {
			return
		}
		factories := map[bool]*goke.Factory{false: si.NewFactory(&h.pos, &h.tag, &h.z, &h.owners), true: si.NewFactory(&h.pos)}
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
						h.tag.Slice(&f.Cursor)[j] = tag.Tags[Family](0).With(h.tags.Selectable)
						h.z.Slice(&f.Cursor)[j] = world.Z{Altitude: spec.alt}
						if spec.owner != control.Nobody {
							h.owners.Slice(&f.Cursor)[j] = tag.Tags[owner.Family](0).With(owner.Of(spec.owner))
						}
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
	events.AddClickEvent(x, y, control.MouseButtonLeft, control.ActionPress)
	events.AddClickEvent(x, y, control.MouseButtonLeft, control.ActionRelease)
	h.handler.HandleEvents(events)
	h.ecs.Tick(time.Second)
}

func (h *harness) drag(x0, y0, x1, y1 int, shift bool) {
	events := &control.InputEvents{}
	events.Modifiers.Shift = shift
	events.AddClickEvent(x0, y0, control.MouseButtonLeft, control.ActionPress)
	events.AddClickEvent(x1, y1, control.MouseButtonLeft, control.ActionRelease)
	h.handler.HandleEvents(events)
	h.ecs.Tick(time.Second)
}

func (h *harness) press(key control.Key) {
	events := &control.InputEvents{}
	events.AddKeyEvent(key, control.ActionPress)
	h.handler.HandleEvents(events)
	h.ecs.Tick(time.Second)
}

// moveTo puts id's box with its top-left at (x, y).
func (h *harness) moveTo(id uid.UID64, x, y float64) {
	for h.moveQ.All(); h.moveQ.Next(); {
		cur := h.moveQ.Cursor()
		for i, got := range cur.IDs {
			if got == id {
				pos := &h.moveBase.Slice(cur)[i].Pos
				pos.AABB = plane.NewAABB(geom.NewVec(x, y), pos.Size.X, pos.Size.Y)
			}
		}
	}
}

func (h *harness) has(id uid.UID64, tag tag.Tag[Family]) bool {
	h.selectedQ.All()
	for h.selectedQ.Next() {
		cur := h.selectedQ.Cursor()
		for i, got := range cur.IDs {
			if got == id {
				return h.marks.Slice(cur)[i].Has(tag)
			}
		}
	}
	return false
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
	press.AddClickEvent(40, 40, control.MouseButtonLeft, control.ActionPress)
	h.handler.HandleEvents(press)
	h.ecs.Tick(time.Second)

	if h.isSelected(*id) {
		t.Fatal("sanity check failed: press alone (no release yet) should not select anything")
	}

	release := &control.InputEvents{}
	release.AddClickEvent(60, 60, control.MouseButtonLeft, control.ActionRelease)
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

func TestMarquee_ShowsTheBoxBeingDraggedUntilItsSelect(t *testing.T) {
	h := newHarness(t)
	h.start()
	box := func() (geom.AABB, bool) { b, ok := h.sel.marquees.boxes[h.local.Camera]; return b, ok }

	press := &control.InputEvents{MousePos: geom.NewVec(10, 10)}
	press.AddClickEvent(10, 10, control.MouseButtonLeft, control.ActionPress)
	h.handler.HandleEvents(press)
	h.ecs.Tick(time.Second)
	if _, ok := box(); ok {
		t.Fatal("a box shows before the cursor moved")
	}

	h.handler.HandleEvents(&control.InputEvents{MousePos: geom.NewVec(40, 60), CursorDelta: geom.NewVec(30, 50)})
	h.ecs.Tick(time.Second)
	if b, ok := box(); !ok || b != control.ScreenRect(geom.NewVec(10, 10), geom.NewVec(40, 60)) {
		t.Fatalf("while dragging the box is %v (shown %v), want from (10,10) to (40,60)", b, ok)
	}

	release := &control.InputEvents{MousePos: geom.NewVec(40, 60)}
	release.AddClickEvent(40, 60, control.MouseButtonLeft, control.ActionRelease)
	h.handler.HandleEvents(release)
	h.ecs.Tick(time.Second)
	if _, ok := box(); ok {
		t.Error("the box still shows after the Select that ended the drag")
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

// standing is a Look drawing an entity upright over its box, lifted by its altitude, as a view with
// heights does.
type standing struct{}

func (standing) Sprite(*render.Frame, camera.Camera, plane.AABB, world.Z, render.AtlasSource, render.Appearance, render.Light) {
}

func (standing) Drawn(cam camera.Camera, box geom.AABB, z world.Z) render.Corners {
	alt := float32(z.Altitude)
	x0, y0 := cam.ToScreen(float32(box.TopLeft.X), float32(box.TopLeft.Y))
	x1, y1 := cam.ToScreen(float32(box.BottomRight.X), float32(box.BottomRight.Y))
	return render.Corners{{x0, y0 - alt}, {x1, y0 - alt}, {x0, y1 - alt}, {x1, y1 - alt}}
}

func (standing) Footprint(cam camera.Camera, box geom.AABB, alt float32, dst []render.Corners) []render.Corners {
	return append(dst, render.ProjectCorners(cam, float32(box.TopLeft.X), float32(box.TopLeft.Y), float32(box.BottomRight.X), float32(box.BottomRight.Y), alt))
}

func TestSystem_Update_ClickPicksWhereTheLookDrawsTheEntity(t *testing.T) {
	h := newHarnessViewed(t, world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Camera:   camera.Config{ViewportWidth: 800, ViewportHeight: 600},
		Heights:  true,
	}, func(w *world.Plugin) { w.SetLook(standing{}) })
	hawk := h.seedHigh(500, 500, 10, 40)
	walker := h.seed(560, 560, 10)
	h.start()
	cam := h.local.Camera
	cam.MoveTo(300, 300)

	// The hawk is drawn 40 up over its box; a click there selects it.
	sx, sy := cam.ToScreen(505, 505)
	h.click(int(sx), int(sy)-40, false)
	if !h.isSelected(*hawk) || h.isSelected(*walker) {
		t.Errorf("clicking the hawk where it is drawn: hawk %v, walker %v; want the hawk alone", h.isSelected(*hawk), h.isSelected(*walker))
	}
	// A click on the ground under the hawk hits nothing.
	h.click(int(sx), int(sy), false)
	if h.isSelected(*hawk) {
		t.Error("clicking the ground under the hawk selected it")
	}
	// The walker on the ground is where its box is.
	wx, wy := cam.ToScreen(565, 565)
	h.click(int(wx), int(wy), false)
	if !h.isSelected(*walker) {
		t.Error("clicking the walker where it stands did not select it")
	}
}
