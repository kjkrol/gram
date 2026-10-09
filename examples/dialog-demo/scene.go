package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

// mainScene is the demo's one scene, built out of the arena's plugins once the world is there.
type mainScene struct{ arena *arena }

// The scene's colours: the meadow, the traveller and the host.
var (
	meadowColor    = color.RGBA{R: 70, G: 110, B: 60, A: 255}
	travellerColor = color.RGBA{R: 230, G: 200, B: 90, A: 255}
	hostColor      = color.RGBA{R: 120, G: 160, B: 230, A: 255}
)

// picture dresses the meadow and the two and hands the world's picture.
func (m *mainScene) picture() render.Picture {
	s := m.arena
	worldAtlas := render.NewAtlas()
	worldAtlas.Add(kind.Named[unitRow](s.world.Kinds(), TravellerKind), UnitSize, render.Diamond(travellerColor))
	worldAtlas.Add(kind.Named[unitRow](s.world.Kinds(), HostKind), UnitSize, render.Solid(hostColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(MeadowCell, render.Solid(meadowColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)
	return render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())
}

// screen is the meadow through a camera of its own, the player's view; above every host that
// talks, its conversation, and under the one the cursor points at, what it makes of the traveller.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).Input(s.players.Through(s.player)),
		s.dialog.Window().Above().Offset(0, -6),
		ui.LabelOf(ui.Of(func(r *Regard) (string, bool) { // what the host makes of the traveller
			chosen, ok := s.selection.Chosen(s.player.ID)
			return r.Stance(chosen), ok
		})).Where(ui.Tagged(s.selection.Tags().Hovered)).Below().Offset(0, 4),
	)
}
