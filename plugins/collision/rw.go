package collision

// Read reads the boxes' drawing, the same on every goroutine.
type Read struct{ p *Plugin }

// Read reads the boxes' drawing on worker w of the game's pool, 0 the game's own goroutine.
func (p *Plugin) Read(int) *Read { return &p.read }

// Write changes the boxes' drawing, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the boxes' drawing, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// BoxesShown is what is drawn of the entities' boxes.
func (r *Read) BoxesShown() BoxesShown { return r.p.Res.Settings.Boxes }

// ShowBoxes has the entities' boxes drawn as shown says (Settings.Boxes), as the game starts.
func (w *Write) ShowBoxes(shown BoxesShown) { w.p.Res.Settings.Boxes = shown }
