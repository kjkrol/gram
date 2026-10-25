package telemetry

import "time"

// Refresh has p read its lines anew every d; zero, at every read.
func Refresh(p *Plugin, d time.Duration) { p.every = d }

// Aged moves p's last reading of its lines d back in time.
func Aged(p *Plugin, d time.Duration) { p.readAt = p.readAt.Add(-d) }
