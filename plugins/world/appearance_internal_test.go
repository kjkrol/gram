package world

import (
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/render"
)

// drawThroughHost runs the Drawing rules over one entity moving with vel and returns its layers.
func drawThroughHost(t *testing.T, vel Velocity, rules ...plugin.Rule) []Appearance {
	t.Helper()
	host := &host.EachHost[Drawing]{}
	for _, b := range rules {
		if err := host.Add(b); err != nil {
			t.Fatal(err)
		}
	}
	layers := []Appearance{{}}
	var base goke.Comp[Base]
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		qb := si.NewQueryBuilder(&base)
		host.Bind(qb)
		q := qb.Build()
		f := si.NewFactory(&base)
		f.Create(1)
		for f.Next() {
			base.Slice(&f.Cursor)[0] = Base{Vel: vel}
		}
		for q.All(); q.Next(); {
			cur := q.Cursor()
			bases := base.Slice(cur)
			host.Run(plugin.Tick{}, cur, func(i int) Drawing { return Drawing{ID: cur.IDs[i], Base: &bases[i], Layers: &layers} })
		}
	}})
	return layers
}

func TestDrawFacing_PicksTheSpriteFromTheEntitysHeading(t *testing.T) {
	const eastward, other render.SpriteID = 7, 3
	facing := Draw.Facing(func(v Velocity) render.SpriteID {
		if v.Dir.X > 0 {
			return eastward
		}
		return other
	})
	for heading, want := range map[string]struct {
		vel  Velocity
		want render.SpriteID
	}{
		"east":  {Velocity{Dir: east, Value: 1}, eastward},
		"north": {Velocity{Dir: north, Value: 1}, other},
	} {
		if got := drawThroughHost(t, want.vel, facing)[0].SpriteID; got != want.want {
			t.Errorf("heading %s: SpriteID = %v, want %v", heading, got, want.want)
		}
	}
}

func TestDrawing_OverlayAsWithShapeTheLayers(t *testing.T) {
	layers := []Appearance{{SpriteID: 1}}
	d := Drawing{Layers: &layers}
	d.Overlay(Appearance{SpriteID: 9})
	d.As(Appearance{SpriteID: 2})
	d.With(func(a Appearance) Appearance { a.SpriteID++; return a })
	if len(layers) != 2 || layers[0].SpriteID != 3 || layers[1].SpriteID != 9 {
		t.Errorf("layers = %v, want the own sprite 2+1 under an overlay of 9", layers)
	}
}
