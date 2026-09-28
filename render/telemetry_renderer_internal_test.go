package render

import (
	"testing"

	"github.com/kjkrol/goke/v3"
)

// lines is a Reporter of fixed lines.
type lines [][2]string

func (lines) Init(*goke.SysInit) {}
func (l lines) Report(line func(label, value string)) {
	for _, p := range l {
		line(p[0], p[1])
	}
}

func TestTelemetryRenderer_AddsTheLinesOfItsReportersInOrder(t *testing.T) {
	r := (&TelemetryRenderer{}).With(lines{{"Time of day", "14:05"}}, lines{{"Wind", "none"}, {"Sea", "calm"}})
	got := string(r.reported([]byte("FPS: 60")))
	if want := "FPS: 60\nTime of day: 14:05\nWind: none\nSea: calm"; got != want {
		t.Errorf("telemetry reads %q, want %q", got, want)
	}
}
