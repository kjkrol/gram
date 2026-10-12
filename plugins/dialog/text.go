package dialog

import (
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// SpeakerText is who speaks in the conversation of the entity an element is pinned to.
func (p *Plugin) SpeakerText() ui.Text {
	return nodeText{p: p, say: func(n *node, _ []*choice) (string, bool) { return n.speaker, true }}
}

// LineText is what is said in the conversation of the entity an element is pinned to.
func (p *Plugin) LineText() ui.Text {
	return nodeText{p: p, say: func(n *node, _ []*choice) (string, bool) { return n.line, true }}
}

// ChoiceText is the words of the answer offered at index in the conversation of the entity an
// element is pinned to; nothing past the last offered.
func (p *Plugin) ChoiceText(index int) ui.Text {
	return nodeText{p: p, say: func(_ *node, offered []*choice) (string, bool) {
		if index >= len(offered) {
			return "", false
		}
		return offered[index].text, true
	}}
}

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
