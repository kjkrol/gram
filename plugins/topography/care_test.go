package topography_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
)

func TestPlugin_ShadesTheGroundWithFullCareAsFarAsTheConfigSays(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{Care: 2})
	if p.Care() != 2 {
		t.Errorf("full care within %d sectors, want the Config's 2", p.Care())
	}
	bound := map[string]bool{}
	for _, bd := range p.DefaultBindings() {
		bound[players.Written(bd.Trigger)] = true
	}
	if !bound["Alt+Shift+="] || !bound["Alt+Shift+-"] {
		t.Errorf("bound %v, want Alt+Shift+= and Alt+Shift+- among them", bound)
	}
}

func TestPlugin_CareStepsFullCareFartherAndNearerRungByRung(t *testing.T) {
	qw := topotest.NewQuasiWorld(t, false,
		func(*board.Units[topotest.Recruit], grid.Grid) []kind.Entry { return nil })
	var caring command.CommandQueue
	for _, q := range qw.Topo.Queues() {
		if q.Accepts() == reflect.TypeFor[topography.Care]() {
			caring = q
		}
	}
	if caring == nil {
		t.Fatal("no queue takes a Care command")
	}
	step := func(by int) int {
		caring.Put(command.Nobody, topography.Care{Step: by})
		qw.ECS.Tick(time.Second / 60)
		return qw.Topo.Care()
	}
	if qw.Topo.Care() != 0 {
		t.Fatalf("full care within %d sectors at first, want everywhere", qw.Topo.Care())
	}
	// nearer from everywhere: 3, 2, 1 and no nearer; farther: 2, 3, everywhere and no farther
	for _, want := range []int{3, 2, 1, 1} {
		if got := step(-1); got != want {
			t.Errorf("nearer: full care within %d sectors, want %d", got, want)
		}
	}
	for _, want := range []int{2, 3, 0, 0} {
		if got := step(1); got != want {
			t.Errorf("farther: full care within %d sectors, want %d", got, want)
		}
	}
}
