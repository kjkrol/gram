package view

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
)

var _ goke.System = (*System)(nil)

// System refreshes every View listed at views once a tick from the Space.
type System struct {
	space     *aabbworld.Space
	views     *[]*View
	worldArea float64
}

// NewSystem builds the system over space, refreshing the Views listed at views.
func NewSystem(space *aabbworld.Space, views *[]*View, worldW, worldH uint32) *System {
	return &System{space: space, views: views, worldArea: float64(worldW) * float64(worldH)}
}

func (s *System) Init(*goke.SysInit) {}

func (s *System) Update(*goke.CmdBuf, time.Duration) {
	for _, v := range *s.views {
		v.refresh(s.space, s.worldArea)
	}
}
