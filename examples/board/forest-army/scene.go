package main

import (
	"fmt"
	"image/color"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

type mainScene struct {
	arena  *arena
	status *status
}

var (
	grassColor  = color.RGBA{R: 105, G: 140, B: 75, A: 255}
	waterColor  = color.RGBA{R: 45, G: 95, B: 165, A: 255}
	fordColor   = color.RGBA{R: 90, G: 135, B: 170, A: 255}
	wallColor   = color.RGBA{R: 70, G: 65, B: 60, A: 255}
	floorColor  = color.RGBA{R: 65, G: 100, B: 50, A: 255}
	crownColor  = color.RGBA{R: 30, G: 95, B: 45, A: 255}
	leafColor   = color.RGBA{R: 70, G: 135, B: 60, A: 255}
	redColor    = color.RGBA{R: 215, G: 60, B: 50, A: 255}
	redWounded  = color.RGBA{R: 120, G: 35, B: 30, A: 255}
	blueColor   = color.RGBA{R: 60, G: 110, B: 220, A: 255}
	blueWounded = color.RGBA{R: 35, G: 60, B: 120, A: 255}
	rimColor    = color.RGBA{R: 245, G: 240, B: 225, A: 255}
	panelColor  = color.RGBA{R: 20, G: 20, B: 20, A: 180}
	minimapRim  = color.RGBA{R: 30, G: 30, B: 30, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	cam := a.cameras.Write().New(cameras.FromAbove,
		camera.Config{Zoom: StartZoom, Follow: RedArmyName})
	return ui.Layers(
		ui.Image(render.NewFeed(cam, a.world.Read(0).Picture())).Input(a.players.Through(a.player)),
		ui.Drawer(a.world.Read(0).Clock().HUD()),
		ui.TopMiddle(ui.LabelOf(m.status).Padding(6).Fill(panelColor).Passing()).Margin(8),
		ui.TopRight(ui.Layers(
			ui.Center(a.minimap.For(cam, a.player.ID).Masked(ui.Circle).Border(minimapRim, 2)).
				Size(220, 220),
			a.minimap.Compass(cam),
		)).Size(220+2*minimap.Overhang, 220+2*minimap.Overhang).Margin(16-minimap.Overhang),
	)
}

// status words how the armies stand, how many of their soldiers and of the forest's trees are
// out, and what the routing did on the abstract graph; read after the ticks, on the HUD and on
// the telemetry panel, which readies it
type status struct {
	arena  *arena
	wholes *goke.Query
	whole  goke.Comp[world.Base]
	parts  *goke.Query
	part   goke.Comp[world.Base]
	losses [soldiers]effect.Effect
	kinds  struct{ red, blue, redMen, blueMen, tree kind.ID }
	last   tally
	text   string
}

// a count of what the status words: an army's strength -1 once it is gone
type tally struct {
	red, blue, redMen, blueMen, trees int
	abstracts, snaps                  uint64
}

func (s *status) Init(si *goke.SysInit) {
	s.wholes = si.NewQueryBuilder(&s.whole).Include(goke.Include[world.Aggregate]()).Build()
	s.parts = si.NewQueryBuilder(&s.part).Include(goke.Include[world.PartOf]()).Build()
	w := s.arena.world
	for k := 1; k < soldiers; k++ {
		s.losses[k] = w.Write().Effects().Named(lossEf(k))
	}
	s.kinds.red = kind.Named[armyRow](w.Kinds(), RedArmyKind).ID()
	s.kinds.blue = kind.Named[armyRow](w.Kinds(), BlueArmyKind).ID()
	s.kinds.redMen = kind.Named[soldierRow](w.Kinds(), RedSoldierKind).ID()
	s.kinds.blueMen = kind.Named[soldierRow](w.Kinds(), BlueSoldierKind).ID()
	s.kinds.tree = kind.Named[treeRow](w.Kinds(), TreeKind).ID()
}

func (s *status) count() tally {
	t := tally{red: -1, blue: -1}
	if s.wholes == nil {
		return t
	}
	for s.wholes.All(); s.wholes.Next(); {
		cur := s.wholes.Cursor()
		for i, b := range s.whole.Slice(cur) {
			switch b.TypeID {
			case s.kinds.red:
				t.red = s.strength(cur.IDs[i])
			case s.kinds.blue:
				t.blue = s.strength(cur.IDs[i])
			}
		}
	}
	for s.parts.All(); s.parts.Next(); {
		for _, b := range s.part.Slice(s.parts.Cursor()) {
			switch b.TypeID {
			case s.kinds.redMen:
				t.redMen++
			case s.kinds.blueMen:
				t.blueMen++
			case s.kinds.tree:
				t.trees++
			}
		}
	}
	c := s.arena.nav.Read(0).Counts()
	t.abstracts, t.snaps = c.Abstracts, c.Snaps
	return t
}

func (s *status) strength(id uid.UID64) int {
	fx := s.arena.world.Read(0).Effects()
	for k := 1; k < soldiers; k++ {
		if fx.Has(id, s.losses[k]) {
			return soldiers - k
		}
	}
	return soldiers
}

func (s *status) Text(uid.UID64, bool) (string, bool) {
	t := s.count()
	if s.text != "" && t == s.last {
		return s.text, true
	}
	s.last = t
	s.text = fmt.Sprintf("Red army %s, %d out   Blue army %s, %d out   Trees %d   "+
		"Far marches %d, snaps %d", strong(t.red), t.redMen, strong(t.blue), t.blueMen,
		t.trees, t.abstracts, t.snaps)
	return s.text, true
}

func (s *status) Report(line func(label, value string)) {
	t := s.count()
	line("Armies", fmt.Sprintf("red %s, %d soldiers out; blue %s, %d out", strong(t.red),
		t.redMen, strong(t.blue), t.blueMen))
	line("Forest", fmt.Sprintf("%d trees", t.trees))
}

func strong(n int) string {
	if n < 0 {
		return "gone"
	}
	return fmt.Sprintf("%d strong", n)
}
