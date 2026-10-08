package dialog

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// SpeakerText is who speaks in the conversation of the entity an element is pinned to, at the
// node it stands at: a window's title.
func (p *Plugin) SpeakerText() ui.Text {
	return nodeText{p: p, say: func(n *node, _ []*choice) (string, bool) { return n.speaker, true }}
}

// LineText is what is said there, line under line.
func (p *Plugin) LineText() ui.Text {
	return nodeText{p: p, say: func(n *node, _ []*choice) (string, bool) { return n.line, true }}
}

// ChoiceText is the words of the answer offered there at index — nothing past the last offered,
// so a button of none is left out.
func (p *Plugin) ChoiceText(index int) ui.Text {
	return nodeText{p: p, say: func(_ *node, offered []*choice) (string, bool) {
		if index >= len(offered) {
			return "", false
		}
		return offered[index].text, true
	}}
}

// nodeText is a Text of the node the pinned entity's conversation stands at.
type nodeText struct {
	p   *Plugin
	say func(n *node, offered []*choice) (string, bool)
}

func (t nodeText) Text(of uid.UID64, pinned bool) (string, bool) {
	if !pinned || t.p.module == nil {
		return "", false
	}
	s := t.p.module.sys
	talk, ok := s.read(of)
	if !ok || !talk.talking() {
		return "", false
	}
	n := t.p.nodes[talk.Node]
	if n == nil {
		return "", false
	}
	return t.say(n, s.offers(nil, n, of, talk.With))
}

// Chooser is who knows a player's one chosen unit: the selection (selection.Plugin.Chosen).
type Chooser interface {
	Chosen(by control.PlayerID) (uid.UID64, bool)
}

// Stance is what the entity an element is pinned to makes of the unit player has chosen
// (Chooser): Friend, Neutral or Enemy by its mood (Config). Nothing while the player has chosen
// none, or that one itself.
func (p *Plugin) Stance(by Chooser, player control.PlayerID) ui.Text {
	return stanceText{p: p, by: by, player: player}
}

type stanceText struct {
	p      *Plugin
	by     Chooser
	player control.PlayerID
}

func (t stanceText) Text(of uid.UID64, pinned bool) (string, bool) {
	if !pinned || t.p.module == nil {
		return "", false
	}
	chosen, ok := t.by.Chosen(t.player)
	if !ok || chosen == of {
		return "", false
	}
	mood, _ := t.p.module.sys.memoryRead(of).of(chosen)
	return t.p.stance(mood), true
}
