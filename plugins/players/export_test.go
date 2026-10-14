package players

import "time"

// CaptureWith has p catch and let go of the cursor through set instead of the window's.
func CaptureWith(p *Plugin, set func(on bool)) { p.setCapture = set }

// ClockWith has p measure how long a key is held by now.
func ClockWith(p *Plugin, now func() time.Time) { p.now = now }
