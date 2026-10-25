package gravity

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type whence struct {
	others *goke.Query
	obase  goke.Comp[world.Base]
}

func (w *whence) init(si *goke.SysInit) { w.others = si.NewQueryBuilder(&w.obase).Build() }

func (w *whence) away(id uid.UID64, a Away) (geom.Vec, bool) {
	if !w.others.Seek(id) {
		return geom.Vec{}, false
	}
	b := w.obase.At(w.others.Cursor())
	at, facing := b.Pos.Center(), b.Vel.Dir
	if a.aimed && a.from != id && w.others.Seek(a.from) {
		from := w.obase.At(w.others.Cursor()).Pos.Center()
		if l := f32.Hypot(at.X-from.X, at.Y-from.Y); l > 0 {
			return geom.NewVec((at.X-from.X)/l, (at.Y-from.Y)/l), true
		}
	}
	return geom.NewVec(-facing.X, -facing.Y), true
}

func pullOf(q *goke.Query, pull *goke.Comp[Pull]) Pull {
	if q.All().Next() {
		return pull.Slice(q.Cursor())[0]
	}
	return Pull{}
}
