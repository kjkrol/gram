package world

import (
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Read reads the world's state on one goroutine at a time: Plugin.Read(w), worker w's.
type Read struct {
	p       *Plugin
	clock   *clock.Read
	effects *cgoke.EffectsRead
}

var _ parallel.Forkable[*Read] = (*Read)(nil)

// Fork reads the same state through queries of its own, for another goroutine.
func (r *Read) Fork() *Read {
	return &Read{p: r.p, clock: r.clock.Fork(), effects: r.effects.Fork()}
}

// Space is the world's spatial index of its entities, to query.
func (r *Read) Space() *aabbworld.SpaceRead { return r.p.module.space.Read() }

// Clock is the world's tactical clock, which everything that simulates goes by.
func (r *Read) Clock() *clock.Read { return r.clock }

// Effects is which effects the entities are under.
func (r *Read) Effects() *cgoke.EffectsRead { return r.effects }

// Tick is the Tick a plugin hands the rules it hosts for a pass over d of the simulation.
func (r *Read) Tick(d time.Duration) host.Tick { return r.p.module.tick(d) }

// HasHeights reports whether this world has heights.
func (r *Read) HasHeights() bool { return r.p.Res.Config.Heights }

// Scale is how many metres a world unit spans.
func (r *Read) Scale() Scale { return r.p.Res.Config.Scale }

// GroundAt is the ground's height under the world point at, as the plugin giving the ground has
// it (Write.SetGround); 0 in a world without one.
func (r *Read) GroundAt(at geom.Vec) float32 { return r.p.module.groundAt(at) }

// Lights are the lights the frame drawn is lit by, the light plugin's; nil without one, or before
// a frame was published.
func (r *Read) Lights() *render.Lights {
	if r.p.lights == nil {
		return nil
	}
	return r.p.lights()
}

// Look is how the world's entities lie on the screen.
func (r *Read) Look() Look { return r.p.look }

// FlatLook is the world's own Look, seen from above.
func (r *Read) FlatLook() Look { return flatLook(r.p.Res.Config.Space) }

// ViewFor is the View of what cam saw as the frame drawn was published, for what draws: nil for
// a camera the frame holds none of; before the first frame published, as the last tick left it.
// A camera's copy has its View.
func (r *Read) ViewFor(cam camera.Camera) *view.View {
	cam = camera.OriginOf(cam)
	if vs := r.p.renderer.figures.Front().views; vs != nil {
		return vs[cam]
	}
	return r.p.views[cam]
}

// Views are the Views of every camera Write.NewView made, as the last tick left them: what
// publishes for a frame culls by them.
func (r *Read) Views() []*view.View { return r.p.module.views }

// InView reports whether some camera's View holds id, as the last tick left them; with no
// camera, everything is.
func (r *Read) InView(id uid.UID64) bool {
	views := r.p.module.views
	if len(views) == 0 {
		return true
	}
	for _, v := range views {
		if v.Contains(id) {
			return true
		}
	}
	return false
}

// Picture is everything the Stage's plugins draw, one picture for every scene showing the world;
// nil before the plugins have declared their looks.
func (r *Read) Picture() render.Picture {
	if r.p.picture == nil {
		return nil
	}
	return r.p.picture
}

// Sheet is the sheets of what stands, images over the whole world from its top-left corner: at
// L4 the albedo, premultiplied, and the relief (the top's height a float16, its low byte in red
// and its high in green, its normal's x and y in blue and alpha as (n+1)/2), perUnit texels a
// world unit, and their halves at L5. They are nil until a renderer asks for them
// (Plugin.Sheets) and in a world without sectors, perUnit 0 while no sector holds a photograph of
// something; for what draws, as the frame drawn holds them.
func (r *Read) Sheet() (albedo, relief, albedoL5, reliefL5 *render.Image, perUnit float32) {
	s := &r.p.renderer.sheet
	live := s.live()
	if f := &r.p.renderer.figures.Front().sheet; f.on {
		live = f.live
	}
	if live {
		perUnit = s.perUnit
	}
	return s.albedo, s.relief, s.albedoL5, s.reliefL5, perUnit
}

// Painted is the version of the photograph of what stands in sector i now its sheet holds,
// something standing there, 0 for none, new as the photograph changes: for what draws, as the
// frame drawn holds it.
func (r *Read) Painted(i int) uint32 {
	if f := &r.p.renderer.figures.Front().sheet; f.on {
		if i < 0 || i >= len(f.held) {
			return 0
		}
		return f.held[i]
	}
	return r.p.renderer.sheet.held(i)
}

// FarLook is how the loose units are drawn at the strategic scale: as the Settings say, else as
// the Config does, else Points.
func (r *Read) FarLook() FarLook {
	if f := r.p.Res.Settings.Far; f != AsConfig {
		return f
	}
	if f := r.p.Res.Config.Far; f != AsConfig {
		return f
	}
	return Points
}

// Counts reports whether the number of a kind's units in a sector is drawn by them at the
// strategic scale: as the Settings say, else as the Config does.
func (r *Read) Counts() bool {
	switch r.p.Res.Settings.Counts {
	case On:
		return true
	case Off:
		return false
	}
	return r.p.Res.Config.Counts
}

// Banner is a kind's loose units in a sector drawn as one at the strategic scale: the entities
// IDs, standing for Count units (an aggregate folded for its Strength), drawn over Box round their
// centroid, Center, at their mean Z, under a badge reaching Reach shown pixels round its middle
// (BadgeAt).
type Banner struct {
	Kind   kind.ID
	Sector int
	Center geom.Vec
	IDs    []uid.UID64
	Count  int
	Box    geom.AABB
	Z      Z
	Reach  float32
}

// BadgeAt is where cam shows b's badge, in its pixels: its middle and how far it reaches round it.
func (b Banner) BadgeAt(cam camera.Camera) (x, y, reach float32) {
	shown := camera.ResolutionOf(cam)
	x, y = cam.Project(b.Center.X, b.Center.Y, b.Z.Top())
	return x, y - badgeLift(b.Reach)*shown, b.Reach * shown
}

// Banners appends to dst the banners cam drew in the frame drawn, where the far units are drawn
// as Banners; their IDs are the frame's, good till the next frame is drawn.
func (r *Read) Banners(cam camera.Camera, dst []Banner) []Banner {
	return r.p.renderer.bannersOf(cam, dst)
}

// Atlas is the world's looks as Write.SetAtlas took them, nil before.
func (r *Read) Atlas() render.AtlasSource { return r.p.renderer.atlas }

// Styles are the styles the figures declared on the world's atlas (Slot.Styled), for the
// plugins drawing about a kind; none before the atlas is closed.
func (r *Read) Styles() *render.Styles { return r.p.styles }

// Banded reports whether the sectors' levels are ticked; true in a world of no sectors.
func (r *Read) Banded() bool { return !r.p.Res.Settings.Unbanded }

// LevelAt is the level the sector holding the world point (x, y) is ticked at, as the last tick
// left it; Standard in a world of no sectors and with the levels switched off.
func (r *Read) LevelAt(x, y float32) lod.Level { return r.p.module.sectors.levelAt(x, y) }

// SectorOf is the sector holding the world point (x, y), held to the world's edges; 0 in a world
// of no sectors.
func (r *Read) SectorOf(x, y float32) int { return r.p.module.sectors.of(x, y) }

// SectorSide is the side of the world's square sectors, in world units; 0 in a world of no
// sectors.
func (r *Read) SectorSide() float32 {
	if g := r.p.module.sectors.grid; g != nil {
		return g.Side()
	}
	return 0
}

// Small is the longer side a thing standing for good may have and still live by its centre
// (Config.Sectors.Small, a quarter of a sector's side unless set); 0 in a world of no sectors.
func (r *Read) Small() float32 {
	g := r.p.module.sectors.grid
	switch {
	case g == nil:
		return 0
	case r.p.Res.Config.Sectors.Small > 0:
		return r.p.Res.Config.Sectors.Small
	}
	return g.Side() / 4
}

// SectorsLen is how many sectors the world is cut into; 0 in a world of no sectors.
func (r *Read) SectorsLen() int {
	if g := r.p.module.sectors.grid; g != nil {
		return g.Len()
	}
	return 0
}

// SectorsShown is what is drawn of the sectors.
func (r *Read) SectorsShown() SectorsShown { return r.p.Res.Settings.Sectors }

// Sectors is the world's part of the telemetry panel: how many sectors and entities lie at each
// level.
func (r *Read) Sectors() render.Reporter { return sectorsReport{r.p.module} }

// Sight is how far from its viewpoint cam sees the world, ticked and drawn: the ladder's Sight
// times the camera's (camera.SightOf); 0 without end.
func (r *Read) Sight(cam camera.Camera) float32 {
	return r.p.Res.Config.LOD.Sight * camera.SightOf(cam)
}

// Seen is the world's part of the telemetry panel about the first camera made: how many sectors
// it sees at each level.
func (r *Read) Seen() render.Reporter { return seenReport{r.p.module} }

// Write changes the world's state, on the game's goroutine.
type Write struct{ p *Plugin }

// Space is the world's spatial index of its entities, indexed again as they spawn, go and move.
func (w *Write) Space() *aabbworld.Space { return w.p.module.space }

// Restill has the space take what stands for good anew once a Caps of it changed in place: its
// still layer holds them as it found them when it was last gathered.
func (w *Write) Restill() {
	x := w.p.module.indexer
	w.p.module.indexStill(x.still, &x.stillBase)
}

// Clock is the world's tactical clock, to hand what simulates and to drive.
func (w *Write) Clock() *clock.Clock { return w.p.module.clock }

// Effects is this Stage's register of effects, which last in the clock's time, to cast and
// dispel.
func (w *Write) Effects() *cgoke.Effects { return w.p.module.effects }

// Carrier is the Stage's one carrier of commands, taking each to its handler's queue.
func (w *Write) Carrier() *command.Carrier { return &w.p.module.commands }

// Seed adds entries to the entities spawned when this Stage starts fresh.
func (w *Write) Seed(entries ...kind.Entry) {
	w.p.must("units seeded", section.Spawn)
	w.p.seeded = append(w.p.seeded, entries...)
}

// Spawn gives the command [Spawn] for each entry as command.Nobody.
func (w *Write) Spawn(entries ...kind.Entry) {
	for _, e := range entries {
		w.p.module.spawns.Add(command.Nobody, Spawn{Entry: e})
	}
}

// SeedWhole adds a thing standing in pieces to the entities spawned when this Stage starts fresh:
// whole's handle and every piece, each carrying PieceOf it (SpawnWhole).
func (w *Write) SeedWhole(whole kind.Entry, pieces ...kind.Entry) {
	w.p.must("wholes seeded", section.Spawn)
	w.p.wholes = append(w.p.wholes, SpawnWhole{Whole: whole, Pieces: slices.Clone(pieces)})
}

// SpawnWhole gives the command [SpawnWhole] for whole's handle and its pieces as command.Nobody.
func (w *Write) SpawnWhole(whole kind.Entry, pieces ...kind.Entry) {
	w.p.module.spawnWholes.Add(command.Nobody,
		SpawnWhole{Whole: whole, Pieces: slices.Clone(pieces)})
}

// Despawn takes an entity out of the ECS at the end of the tick, a Whole's handle with its pieces.
func (w *Write) Despawn(cb *goke.CmdBuf, id uid.UID64) { w.p.module.despawn(cb, id) }

// NewView makes the View of what cam sees, kept current from the next tick on; the one made
// before for cam.
func (w *Write) NewView(cam camera.Camera) *view.View {
	p := w.p
	if v, ok := p.views[cam]; ok {
		return v
	}
	if p.views == nil {
		p.views = map[camera.Camera]*view.View{}
	}
	v := view.New(cam.Bounds)
	if p.module.sectors.grid != nil {
		v = view.NewSeen(cam, seenSectors{p.module.sectors})
	}
	p.module.views = append(p.module.views, v)
	p.views[cam] = v
	return v
}

// SetLook has the world's entities drawn, picked and outlined by look, a view plugin's.
func (w *Write) SetLook(look Look) { w.p.look = look }

// SetGround has the ground's height under a world point read off ground (Read.GroundAt), the
// plugin's laying the ground in relief.
func (w *Write) SetGround(ground func(at geom.Vec) float32) { w.p.module.ground = ground }

// SetLighting has the world's entities marked by l as they are published, the light plugin's.
// Before Install.
func (w *Write) SetLighting(l Lighting) { w.p.renderer.lighting = l }

// SetLights has the world's picture lit by the lights lights gives (render.Composer.Light), the
// light plugin's; without them it is drawn in white light. Before Install.
func (w *Write) SetLights(lights func() *render.Lights) { w.p.lights = lights }

// SetAtlas takes atlas as the world's looks for the entity renderer, applying what a *Atlas
// declared; its Close calls it. A foreign render.AtlasSource is given by hand.
func (w *Write) SetAtlas(atlas render.AtlasSource) {
	p := w.p
	if !p.looked {
		p.looked = true
		if wa, ok := atlas.(*Atlas); ok {
			if err := p.module.drawing.Add(wa.swaps()...); err != nil {
				panic(err)
			}
			if err := p.module.drawing.Add(wa.rules(p.module.clock.Read().Shown)...); err != nil {
				panic(err)
			}
		}
	}
	p.renderer.atlas = atlas
	p.renderer.shade(nil, nil)
	p.renderer.lie(nil, nil, nil)
	if wa, ok := atlas.(*Atlas); ok {
		p.renderer.shade(wa.shaded, wa.phased)
		p.renderer.lie(wa.lain(), wa.edged(), wa.stood())
		p.styles = &wa.styles
	}
}

// Observe has the world live round the cameras o gives: each sector is ticked at the fullest
// level one of them sees it at (Config.LOD).
func (w *Write) Observe(o sector.Observers) {
	w.p.module.sectors.observers = append(w.p.module.sectors.observers, o)
}

// SetBanded switches the sectors' levels on, or off with every sector ticked at Standard
// (SwitchBands, Settings.Unbanded); the cameras draw by their levels either way.
func (w *Write) SetBanded(on bool) {
	w.p.Res.Settings.Unbanded = !on
	w.p.module.sectors.unbanded = !on
}

// ShowSectors has the sectors drawn as shown says (Settings.Sectors): a look for finding out
// what lives and what is seen where.
func (w *Write) ShowSectors(shown SectorsShown) {
	w.p.Res.Settings.Sectors = shown
	w.p.module.sectorsShown = shown
}

// ShowFar has the loose units drawn at the strategic scale as look says (ShowFar, Settings.Far),
// AsConfig as the Config does.
func (w *Write) ShowFar(look FarLook) { w.p.Res.Settings.Far = look }

// ShowCounts has the number of a kind's units in a sector drawn by them at the strategic scale,
// or not (ShowCounts, Settings.Counts).
func (w *Write) ShowCounts(on bool) {
	w.p.Res.Settings.Counts = Off
	if on {
		w.p.Res.Settings.Counts = On
	}
}
