package control

import "fmt"

// Key is a key of the keyboard, by its place on it; Shift, Control and Alt are either of the two.
type Key uint16

// The keys, named as Ebitengine named them.
const (
	KeyUnknown Key = iota
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyEscape
	KeyEnter
	KeySpace
	KeyTab
	KeyBackspace
	KeyDelete
	KeyInsert
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyShift
	KeyControl
	KeyAlt
	KeyMinus
	KeyEqual
	KeyBracketLeft
	KeyBracketRight
	KeyBackslash
	KeySemicolon
	KeyQuote
	KeyBackquote
	KeyComma
	KeyPeriod
	KeySlash
	KeyCapsLock
	KeyNumpad0
	KeyNumpad1
	KeyNumpad2
	KeyNumpad3
	KeyNumpad4
	KeyNumpad5
	KeyNumpad6
	KeyNumpad7
	KeyNumpad8
	KeyNumpad9
	KeyNumpadAdd
	KeyNumpadSubtract
	KeyNumpadMultiply
	KeyNumpadDivide
	KeyNumpadDecimal
	KeyNumpadEnter
	keyCount
)

var keyNames = [keyCount]string{
	"Unknown", "A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T",
	"U", "V", "W", "X", "Y", "Z", "Digit0", "Digit1", "Digit2", "Digit3", "Digit4", "Digit5", "Digit6", "Digit7",
	"Digit8", "Digit9", "F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12", "Escape",
	"Enter", "Space", "Tab", "Backspace", "Delete", "Insert", "Home", "End", "PageUp", "PageDown", "ArrowUp",
	"ArrowDown", "ArrowLeft", "ArrowRight", "Shift", "Control", "Alt", "Minus", "Equal", "BracketLeft",
	"BracketRight", "Backslash", "Semicolon", "Quote", "Backquote", "Comma", "Period", "Slash", "CapsLock",
	"Numpad0", "Numpad1", "Numpad2", "Numpad3", "Numpad4", "Numpad5", "Numpad6", "Numpad7", "Numpad8",
	"Numpad9", "NumpadAdd", "NumpadSubtract", "NumpadMultiply", "NumpadDivide", "NumpadDecimal", "NumpadEnter",
}

// String is the key's name, as Ebitengine wrote it: "A", "Digit1", "ArrowUp".
func (k Key) String() string {
	if k < keyCount {
		return keyNames[k]
	}
	return fmt.Sprintf("Key(%d)", k)
}

// Keys is how many keys there are, for going over all of them.
const Keys = int(keyCount)

// MouseButton is a button of the mouse.
type MouseButton uint8

const (
	MouseButtonLeft MouseButton = iota
	MouseButtonRight
	MouseButtonMiddle
)
