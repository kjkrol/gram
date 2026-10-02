package players

// CaptureWith has p catch and let go of the cursor through set instead of the window's.
func CaptureWith(p *Plugin, set func(on bool)) { p.setCapture = set }
