package plugin

import "github.com/kjkrol/goke/v3"

// Bound is what reads the world and so is readied with it, once, as a system is: a source the
// Stage's plugins draw, a scene's drawer, a reporter.
type Bound interface {
	Init(*goke.SysInit)
}
