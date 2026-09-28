package topography_test

import (
	"reflect"
	"testing"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/render"
)

// With Config.Heightfield the plugin has a renderer drawing the ground Direct, hidden until the
// heightfield is shown; shown, the board's Look lays no tile and the renderer draws; G issues the
// command. Without it there is no renderer, no G, and showing does nothing.
func TestPlugin_TheHeightfieldTakesTheTilesPlace(t *testing.T) {
	w := newWorld(0)
	b, _ := levelBoard(w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true, Heightfield: true})
	r, ok := p.Renderer().(render.Direct)
	if !ok || r.Tier() != render.Ground {
		t.Fatalf("the renderer is %T, want a render.Direct at the Ground tier", p.Renderer())
	}
	hidden := r.(interface{ Hidden() bool })
	if p.Look() == board.Nothing || p.HeightfieldShown() || !hidden.Hidden() {
		t.Fatal("the heightfield is shown before it is asked for")
	}
	queued := false
	for _, q := range p.Queues() {
		if q.Accepts() == reflect.TypeFor[topography.Heightfield]() {
			queued = true
		}
	}
	if !queued {
		t.Error("no queue takes the Heightfield command")
	}
	keys := map[string]control.Binding{}
	for _, bd := range p.DefaultBindings() {
		keys[players.Written(bd.Trigger)] = bd
	}
	if bd, ok := keys["G"]; !ok {
		t.Error("G is not bound")
	} else if cmd, _ := bd.Build(control.Context{}); cmd != (topography.Heightfield{}) {
		t.Errorf("G issues %+v, want Heightfield", cmd)
	}
	p.ShowHeightfield(true)
	if p.Look() != board.Nothing || !p.HeightfieldShown() || hidden.Hidden() {
		t.Error("shown, the tiles are still laid or the renderer still hidden")
	}
	p.ShowHeightfield(false)
	if p.Look() == board.Nothing || p.HeightfieldShown() || !hidden.Hidden() {
		t.Error("hidden again, the tiles are not back")
	}

	w2 := newWorld(0)
	b2, _ := levelBoard(w2)
	tiles := topography.NewPlugin(w2, b2, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true})
	if tiles.Renderer() != nil {
		t.Errorf("without Config.Heightfield the renderer is %T, want nil", tiles.Renderer())
	}
	for _, bd := range tiles.DefaultBindings() {
		if players.Written(bd.Trigger) == "G" {
			t.Error("without Config.Heightfield G is bound")
		}
	}
	tiles.ShowHeightfield(true)
	if tiles.HeightfieldShown() || tiles.Look() == board.Nothing {
		t.Error("without Config.Heightfield the heightfield was shown")
	}
}
