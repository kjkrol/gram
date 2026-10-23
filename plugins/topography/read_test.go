package topography_test

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
)

// what a goroutine reads of the cage: whether each point is free, the grain there, the span
type caged struct {
	free      [16 * 16 * 4]bool
	step      [16 * 16]float32
	low, high float32
}

func readCage(r *topography.Read) caged {
	var c caged
	for y := range 16 {
		for x := range 16 {
			fx, fy := float32(x*8+4), float32(y*8+4)
			for z := range 4 {
				c.free[(y*16+x)*4+z] = r.Free(fx, fy, float32(z*10))
			}
			c.step[y*16+x], _ = r.Grain(fx, fy)
		}
	}
	c.low, c.high = r.Span()
	return c
}

func TestRead_TheWorkersReadTheCageAtOnceAsTheGameDoes(t *testing.T) {
	qw := topotest.NewQuasiWorld(t, false, func(*board.Units[topotest.Recruit], grid.Grid) []kind.Entry {
		return nil
	})
	qw.ECS.Tick(time.Second / 60)
	qw.ECS.Tick(time.Second / 60)
	want := readCage(qw.Topo.Read(0))
	if want.high <= 0 || want.free[(1*16+9)*4] {
		t.Fatalf("the game's goroutine reads a span to %v, the hill's foot free: the cage proves "+
			"nothing", want.high)
	}
	pool := topotest.Pool
	got := make([]caged, 2*pool.Size())
	b := pool.Begin(func(w, i int) {
		for range 20 {
			got[i] = readCage(qw.Topo.Read(w))
		}
	}, nil)
	b.Add(len(got))
	b.Wait()
	for i := range got {
		if got[i] != want {
			t.Fatalf("task %d read the cage otherwise than the game's goroutine", i)
		}
	}
}
