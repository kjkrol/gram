package telemetry

import "time"

// Refresh has p's panel build its text anew every d; zero, at every read.
func Refresh(p *Plugin, d time.Duration) { p.panel.every = d }

// Aged moves the last build of p's panel's text d back in time.
func Aged(p *Plugin, d time.Duration) { p.panel.at = p.panel.at.Add(-d) }
