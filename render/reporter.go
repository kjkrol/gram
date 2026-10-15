package render

// Reporter is a plugin's part of the telemetry panel: lines it adds under the standard ones,
// each a label and a value, asked for once a frame.
type Reporter interface {
	Drawer
	Report(line func(label, value string))
}
