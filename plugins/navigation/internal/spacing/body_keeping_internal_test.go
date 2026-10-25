package spacing

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
)

// the one touched lies east, so the way off them is west
func TestClearOf_NeverIntoTheOneTouched(t *testing.T) {
	off := geom.NewVec(-1, 0)
	for name, c := range map[string]struct {
		facing, dir geom.Vec
		wantDir     geom.Vec
		wantSpeed   float32
	}{
		"along them, as it was": {
			facing:    geom.NewVec(0, 1),
			dir:       geom.NewVec(0, 1),
			wantDir:   geom.NewVec(0, 1),
			wantSpeed: 10,
		},
		"slantwise into them, along them only": {
			facing:    geom.NewVec(0, 1),
			dir:       geom.NewVec(1, 1),
			wantDir:   geom.NewVec(0, 1),
			wantSpeed: 10,
		},
		"facing into them, turns on the spot": {
			facing:    geom.NewVec(1, 0),
			dir:       geom.NewVec(0, 1),
			wantDir:   geom.NewVec(0, 1),
			wantSpeed: 0,
		},
		"straight through them, stands": {
			facing:    geom.NewVec(0, 1),
			dir:       geom.NewVec(1, 0),
			wantDir:   geom.Vec{},
			wantSpeed: 0,
		},
		"turning back the short way through them, round their far side": {
			facing:    geom.NewVec(0.006, -1),
			dir:       geom.NewVec(0, 1),
			wantDir:   off,
			wantSpeed: 10,
		},
		"turning back the short way off them, as asked": {
			facing:    geom.NewVec(-0.006, -1),
			dir:       geom.NewVec(0, 1),
			wantDir:   geom.NewVec(0, 1),
			wantSpeed: 10,
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir, speed := clearOf(c.facing, c.dir, 10, off)
			if dir != c.wantDir || speed != c.wantSpeed {
				t.Errorf("clearOf = %v at %v, want %v at %v", dir, speed, c.wantDir, c.wantSpeed)
			}
		})
	}
}
