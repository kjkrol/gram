package motion

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/uid"
)

// Write changes what motion moves, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes what motion moves, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Leave marks the entity id as gone past an open edge, a leaver from the next step on; a plugin
// moving entities itself calls it.
func (w *Write) Leave(cb *goke.CmdBuf, id uid.UID64) { w.p.module.leave(cb, id) }
