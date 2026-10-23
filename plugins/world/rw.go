package world

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/command"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
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

// Lighting is what lights the world's entities, nil for white light.
func (r *Read) Lighting() Lighting { return r.p.renderer.lighting }

// Look is how the world's entities lie on the screen.
func (r *Read) Look() Look { return r.p.look }

// FlatLook is the world's own Look, seen from above.
func (r *Read) FlatLook() Look { return flatLook(r.p.Res.Config.Space) }

// ViewFor is the View of what cam saw as the frame drawn was published, for what draws; before
// the first, as the last tick left it; nil for a camera of none.
func (r *Read) ViewFor(cam camera.Camera) *view.View {
	if v := r.p.renderer.figures.Front().views[cam]; v != nil {
		return v
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

// Atlas is the world's looks as Write.SetAtlas took them, nil before.
func (r *Read) Atlas() render.AtlasSource { return r.p.renderer.atlas }

// Styles are the styles the figures declared on the world's atlas (Slot.Styled), for the
// plugins drawing about a kind; none before the atlas is closed.
func (r *Read) Styles() *render.Styles { return r.p.styles }

// Banded reports whether the sectors' bands are on; true in a world of no sectors.
func (r *Read) Banded() bool { return !r.p.Res.Settings.Unbanded }

// SectorsShown is what is drawn of the sectors.
func (r *Read) SectorsShown() SectorsShown { return r.p.Res.Settings.Sectors }

// Sectors is the world's part of the telemetry panel: how many sectors and entities lie in each
// band.
func (r *Read) Sectors() render.Reporter { return sectorsReport{r.p.module} }

// Seen is the world's part of the telemetry panel about the first camera made: how many sectors
// it sees at each level.
func (r *Read) Seen() render.Reporter { return seenReport{r.p.module} }

// Write changes the world's state, on the game's goroutine.
type Write struct{ p *Plugin }

// Space is the world's spatial index of its entities, indexed again as they spawn, go and move.
func (w *Write) Space() *aabbworld.Space { return w.p.module.space }

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

// Despawn takes an entity out of the ECS at the end of the tick.
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

// SetLighting has the world's entities lit by l, the light plugin's; without one they are drawn
// in white light. Before Install.
func (w *Write) SetLighting(l Lighting) { w.p.renderer.lighting = l }

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

// Anchor has the world live round the points a gives: the sectors' bands (Config.Sectors) are
// taken from the nearest of them.
func (w *Write) Anchor(a sector.Anchors) {
	w.p.module.sectors.anchors = append(w.p.module.sectors.anchors, a)
}

// SetBanded switches the sectors' bands on, or off with every sector in the first (SwitchBands,
// Settings.Unbanded).
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
