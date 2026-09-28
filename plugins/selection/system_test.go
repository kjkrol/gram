package selection

import (
	"math"
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
	"github.com/kjkrol/gram/render"
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
	world     *world.Plugin
	sys       *SelectionSystem
	follow    *FollowSystem
	handler   control.EventHandler
	ecs       *goke.ECS
	pos       goke.Comp[world.Base]
	z         goke.Comp[world.Z]
	tag       goke.Comp[plugin.Tags[Family]]
	marks     goke.Comp[plugin.Tags[Family]]
	tags      Tags
	selectedQ *goke.Query
	handle    goke.Runnable
	followRun goke.Runnable
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
	tags := Tags{Selectable: 0, Selected: 1, Followed: 2}
	sys := NewSelectionSystem(&sel.selects, space, tags, w.Look)
	follow := NewFollowSystem(&sel.follows, tags)
	sys.marqueeQueue, sys.marquees = &sel.marqueeQueue, &sel.marquees

	return &harness{t: t, world: w, space: space, players: pl, local: local, sel: sel, sys: sys, follow: follow, handler: pl.EventHandler(), ecs: goke.New(), tags: tags}
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
		h.moveQ = si.NewQueryBuilder(&h.moveBase).Build()
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
	h.followRun = h.ecs.RegSys(h.follow)
	h.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(h.handle, d)
		ctx.Run(h.followRun, d)
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

func (h *harness) press(key ebiten.Key) {
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

func (h *harness) has(id uid.UID64, tag plugin.Tag[Family]) bool {
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

// followHarness is a world larger than its 200x200 screen, so the camera has room to follow.
func followHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessIn(t, world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Camera:   camera.Config{ViewportWidth: 200, ViewportHeight: 200},
	})
}

// centred reports whether the camera draws the middle of id's 10x10 box at (x, y) in the middle of its screen.
func centred(h *harness, x, y float64) bool {
	sx, sy := h.local.Camera.Project(float32(x+5), float32(y+5), 0)
	return math.Abs(float64(sx-100)) < 0.5 && math.Abs(float64(sy-100)) < 0.5
}

func TestFollow_FTheOneSelectedUnitAndTheCameraKeepsItInTheMiddle(t *testing.T) {
	h := followHarness(t)
	unit := h.seed(150, 150, 10)
	h.start()
	h.click(155, 155, false)
	h.press(ebiten.KeyC)
	if !h.has(*unit, h.tags.Followed) || !centred(h, 150, 150) {
		t.Fatalf("after C: followed %v, centred %v; want both", h.has(*unit, h.tags.Followed), centred(h, 150, 150))
	}
	h.moveTo(*unit, 500, 420)
	h.ecs.Tick(time.Second)
	if !centred(h, 500, 420) {
		t.Error("the camera did not follow the unit to its new place")
	}
	h.local.Camera.ZoomIn(2, 505, 425)
	h.ecs.Tick(time.Second)
	if !h.has(*unit, h.tags.Followed) || !centred(h, 500, 420) {
		t.Error("zooming ended the following")
	}
	h.press(ebiten.KeyC)
	if h.has(*unit, h.tags.Followed) {
		t.Error("a second F did not stop the following")
	}
}

func TestFollow_MovesTheCameraOfThePlayerWhoAsked(t *testing.T) {
	h := followHarness(t)
	shared := h.local.Camera // the world's
	before := shared.Bounds()
	h.local.OwnCamera()
	unit := h.seed(150, 150, 10)
	h.start()
	h.click(155, 155, false) // picked through the player's own camera, which starts where the world's does
	h.press(ebiten.KeyC)
	if !h.has(*unit, h.tags.Followed) || !centred(h, 150, 150) {
		t.Fatalf("after C: followed %v, centred %v in the player's own camera; want both", h.has(*unit, h.tags.Followed), centred(h, 150, 150))
	}
	if shared.Bounds() != before {
		t.Error("following moved the world's camera, not the one of the player who asked")
	}
}

func TestFollow_MovingTheCameraByHandEndsIt(t *testing.T) {
	h := followHarness(t)
	unit := h.seed(150, 150, 10)
	h.start()
	h.click(155, 155, false)
	h.press(ebiten.KeyC)
	h.local.Camera.Pan(40, 0)
	h.ecs.Tick(time.Second)
	if h.has(*unit, h.tags.Followed) {
		t.Error("the unit is still followed after the player panned the camera away")
	}
}

func TestFollow_SeveralSelectedFollowsNone(t *testing.T) {
	h := followHarness(t)
	a := h.seed(20, 20, 10)
	b := h.seed(60, 60, 10)
	h.start()
	h.drag(10, 10, 90, 90, false)
	if !h.isSelected(*a) || !h.isSelected(*b) {
		t.Fatal("sanity check failed: expected both selected")
	}
	h.press(ebiten.KeyC)
	if h.has(*a, h.tags.Followed) || h.has(*b, h.tags.Followed) {
		t.Error("F followed one of several selected units, want none")
	}
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

func TestMarquee_ShowsTheBoxBeingDraggedUntilItsSelect(t *testing.T) {
	h := newHarness(t)
	h.start()
	box := func() (geom.AABB, bool) { b, ok := h.sel.marquees.boxes[h.local.Camera]; return b, ok }

	press := &control.InputEvents{MousePos: geom.NewVec(10, 10)}
	press.AddClickEvent(10, 10, ebiten.MouseButtonLeft, control.ActionPress)
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
	release.AddClickEvent(40, 60, ebiten.MouseButtonLeft, control.ActionRelease)
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

func (standing) Sprite(*render.Frame, camera.Camera, plane.AABB, world.Z, render.AtlasSource, render.SpriteID, render.Light, float32) {
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
		Quasi3D:  true,
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
