package world

import (
	"testing"
	"time"

	"github.com/kjkrol/ceremony/host"
)

func TestTick_AllocatesNothing(t *testing.T) {
	wm := NewPlugin(Config{
		Space:    SpaceCfg{Width: 1000, Height: 1000},
		Entities: EntitiesCfg{MaxCount: 10, MinSize: 1, MaxSize: 100},
	}).module
	var tick host.Tick
	allocs := testing.AllocsPerRun(100, func() { tick = wm.tick(time.Millisecond) })
	if allocs != 0 || tick.Roles == nil {
		t.Errorf("a host.Tick of the world allocates %v times, roles %v; want none and a reader",
			allocs, tick.Roles != nil)
	}
}
