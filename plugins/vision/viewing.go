package vision

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

// Viewing is what a trigger hosted by the Renderer gets for every observer as the
// frame is composed: Show has its view drawn. With no such trigger registered every observer's
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
func ShowViewOf[F any](t tag.Tag[F]) plugin.Trigger {
	return act.Trigger[Viewing]("show the view").Self(t).Runs(func(_ plugin.Tick, v Viewing) { v.Show() })
}
