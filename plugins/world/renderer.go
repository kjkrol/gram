package world

import (
	"github.com/kjkrol/gram/plugin/host"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

var _ render.Source = (*Renderer)(nil)

// Renderer is the render.Source of the Position+Appearance entities in the View of the viewport's
// camera — what it sees this tick — each laid on the screen by the world's Look, running the Each
// behaviors of a Drawing over each chunk to settle their layers. A Stage that has not ticked yet
// sees everything.
type Renderer struct {
	renderQuery *goke.Query
	base        goke.Comp[Base]
	appearance  goke.Comp[Appearance]
	z           goke.OptComp[Z]
	host        *host.EachHost[Drawing]
	layers      [][]Appearance // one per entity of the chunk being drawn
	atlas       render.AtlasSource
	look        func() Look
	views       func(camera.Camera) *View
	view        *View // the one being drawn

	ids   []uid.UID64
	bases []Base
}

func newRenderer(atlas render.AtlasSource, views func(camera.Camera) *View, host *host.EachHost[Drawing], look func() Look) *Renderer {
	return &Renderer{atlas: atlas, views: views, host: host, look: look}
}

func (s *Renderer) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z)
	s.host.Bind(qb)
	s.renderQuery = qb.Build()
}

// Compose hands f every drawn entity in sight of cam, as the world's Look lays it.
func (s *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	look := s.look()
	s.view = s.views(cam)
	s.each(func(i int, alt float32, sprite render.SpriteID) {
		if box := s.bases[i].Pos.AABB; cam.Visible(box.AABB) {
			look.Sprite(f, cam, box, alt, s.atlas, sprite)
		}
	})
}

// each walks the drawn entities of the View, their Drawing behaviors run, calling draw once per
// layer with the entity's index in the chunk and its altitude.
func (s *Renderer) each(draw func(i int, alt float32, sprite render.SpriteID)) {
	tick := plugin.Tick{Now: time.Now()}
	s.renderQuery.All()
	for s.renderQuery.Next() {
		cursor := s.renderQuery.Cursor()
		s.ids, s.bases = cursor.IDs, s.base.Slice(cursor)
		appearances := s.appearance.Slice(cursor)
		zs := s.z.Slice(cursor)

		for len(s.layers) < len(s.ids) {
			s.layers = append(s.layers, nil)
		}
		for i := range s.ids {
			s.layers[i] = append(s.layers[i][:0], appearances[i])
		}
		if !s.host.Empty() {
			s.host.Run(tick, cursor, s.at)
		}
		for i, id := range s.ids {
			if !s.view.Contains(id) {
				continue
			}
			alt := float32(0)
			if zs != nil {
				alt = float32(zs[i].Altitude)
			}
			for _, l := range s.layers[i] {
				draw(i, alt, l.SpriteID)
			}
		}
	}
}

// at describes the i-th entity of the chunk being drawn.
func (s *Renderer) at(i int) Drawing {
	return Drawing{ID: s.ids[i], Base: &s.bases[i], Layers: &s.layers[i]}
}
