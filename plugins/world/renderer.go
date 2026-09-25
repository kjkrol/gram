package world

import (
	"github.com/kjkrol/gram/plugin/host"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

var _ render.WorldRenderer = (*Renderer)(nil)

// Renderer draws the Position+Appearance entities in the View of the viewport's camera — what it
// sees this tick — running the Each behaviors of a Drawing over each chunk to settle their layers. A
// Stage that has not ticked yet sees everything. Handed to render.NewSorted, it submits each
// entity's box at its Z.Altitude instead, at the depth of its centre.
type Renderer struct {
	renderQuery *goke.Query
	base        goke.Comp[Base]
	appearance  goke.Comp[Appearance]
	z           goke.OptComp[Z]
	host        *host.EachHost[Drawing]
	layers      [][]Appearance // one per entity of the chunk being drawn
	batch       spriteBatch
	atlas       render.AtlasSource
	views       func(camera.Camera) *View
	view        *View // the one being drawn

	ids   []uid.UID64
	bases []Base
}

var _ render.Submitter = (*Renderer)(nil)

func newRenderer(atlas render.AtlasSource, views func(camera.Camera) *View, host *host.EachHost[Drawing], worldW, worldH uint32) *Renderer {
	return &Renderer{batch: newSpriteBatch(atlas, worldW, worldH), atlas: atlas, views: views, host: host}
}

func (s *Renderer) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z)
	s.host.Bind(qb)
	s.renderQuery = qb.Build()
}

// DrawWorld draws this frame through cam; a nil screen gathers the quads and draws nothing, for
// measuring.
func (s *Renderer) DrawWorld(screen *ebiten.Image, cam camera.Camera) {
	s.batch.reset(cam)
	s.view = s.views(cam)
	s.each(func(i int, _ float32, sprite render.SpriteID) { s.batch.drawQuad(s.bases[i].Pos, sprite) })
	s.batch.flush(screen)
}

// Submit hands every drawn entity to sink at the depth of its centre, which ties with the tile it
// stands on and follows it: its box lifted to its altitude, or through an isometric camera a
// billboard the size of its box standing on its centre.
func (s *Renderer) Submit(sink *render.Sink, cam camera.Camera) {
	_, iso := cam.Projection().(camera.Isometric)
	s.view = s.views(cam)
	s.each(func(i int, alt float32, sprite render.SpriteID) {
		box := s.bases[i].Pos.AABB
		if !cam.Visible(box.AABB) {
			return
		}
		x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
		x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
		cx, cy := (x0+x1)/2, (y0+y1)/2
		dst := render.ProjectCorners(cam, x0, y0, x1, y1, alt)
		if iso {
			dst = render.Billboard(cam, cx, cy, alt, x1-x0, y1-y0)
		}
		sink.Quad(cam.Depth(cx, cy, alt), s.atlas, sprite, dst)
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
