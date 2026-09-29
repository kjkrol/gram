package render

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
)

var _ Renderer = (*TelemetryRenderer)(nil)

// TelemetryRenderer prints how the game runs in the corner of the screen: the frame and tick
// rates, the entity count, and the lines of every Reporter it was built With.
type TelemetryRenderer struct {
	measuredTPS *int
	entityCount func() int
	reporters   []Reporter
	text        []byte
}

// NewTelemetryRenderer shows the frame rate, the tick rates and the entity count.
func NewTelemetryRenderer(measuredTPS *int, entityCount func() int) *TelemetryRenderer {
	return &TelemetryRenderer{measuredTPS: measuredTPS, entityCount: entityCount}
}

// With adds the lines of reporters, in order, under the engine's own.
func (s *TelemetryRenderer) With(reporters ...Reporter) *TelemetryRenderer {
	s.reporters = append(s.reporters, reporters...)
	return s
}

func (s *TelemetryRenderer) Init(si *goke.SysInit) {
	for _, r := range s.reporters {
		r.Init(si)
	}
}

func (s *TelemetryRenderer) Draw(screen *Image) {
	s.text = fmt.Appendf(s.text[:0],
		"FPS: %0.2f\nTPS (engine): %0.2f\nTPS (Physics): %d\nEntities: %d",
		ActualFPS(),
		ActualTPS(),
		*s.measuredTPS,
		s.entityCount(),
	)
	s.text = s.reported(s.text)
	DebugPrint(screen, string(s.text))
}

// reported appends to text a line "label: value" for every line of the reporters.
func (s *TelemetryRenderer) reported(text []byte) []byte {
	for _, r := range s.reporters {
		r.Report(func(label, value string) {
			text = append(append(append(append(text, '\n'), label...), ": "...), value...)
		})
	}
	return text
}
