package control

// EventHandler reacts to this tick's input events.
type EventHandler interface {
	HandleEvents(events *InputEvents)
}
