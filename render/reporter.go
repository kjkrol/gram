package render

// Reporter is a plugin's part of a TelemetryRenderer: lines it adds under the engine's own, each a
// label and a value, asked for once a frame. Init runs once, with the telemetry's.
type Reporter interface {
	Layer
	Report(line func(label, value string))
}
