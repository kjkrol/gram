package vision

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Viewing is what an Each or Every behavior hosted by the Renderer gets for every observer as the
// frame is composed: Show has its view drawn. With no such behavior registered every observer's
// view is drawn; with any, only the views one shows. Register it with Plugin.RegisterBehavior.
type Viewing struct {
	ID    uid.UID64
	Base  *world.Base
	shown *bool
}

// Show has the observer's view drawn this frame.
func (v Viewing) Show() { *v.shown = true }

// ShowViewOf shows the view of every observer carrying tag — the ones selected, say, which takes
// in the one ridden in first person too.
func ShowViewOf[F any](tag plugin.Tag[F]) plugin.Behavior {
	return host.Each(func(_ plugin.Tick, tags *plugin.Tags[F], v Viewing) {
		if tags.Has(tag) {
			v.Show()
		}
	})
}
