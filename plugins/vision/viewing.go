package vision

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Viewing is what a rule hosted by the Renderer gets for every observer as the
// frame is composed: Show has its view drawn. With no such rule registered every observer's
// view is drawn; with any, only the views one shows. Register it with Plugin.Hook.
type Viewing struct {
	ID    uid.UID64
	Base  *world.Base
	shown *bool
}

// Show has the observer's view drawn this frame.
func (v Viewing) Show() { *v.shown = true }

// ShowViewOf shows the view of every observer carrying t — the ones selected, say, which takes
// in the one ridden in first person too.
func ShowViewOf[F any](t tag.Tag[F]) plugin.Rule {
	return host.Each(func(_ plugin.Tick, tags *tag.Tags[F], v Viewing) {
		if tags.Has(t) {
			v.Show()
		}
	})
}
