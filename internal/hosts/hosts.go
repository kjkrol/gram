// Package hosts hands rules to the hosts of the moments they are of: what the engine does with
// the roles' rules once a Stage's Init returns, and the tests' installers after it.
package hosts

import (
	"errors"
	"fmt"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule"
)

// Deliver hands each rule — a role's, each of its rules — to the first of among that takes it,
// trying the next while one refuses it with plugin.ErrUnhosted. A rule none takes is
// plugin.ErrUnhosted, named by its String; any other error stops at once.
func Deliver(among []plugin.Host, rules ...rule.Rule) error {
	for _, r := range rules {
		if role, ok := r.(*rule.Part); ok {
			if err := Deliver(among, role.Rules()...); err != nil {
				return err
			}
			continue
		}
		if err := deliver(among, r); err != nil {
			return err
		}
	}
	return nil
}

func deliver(among []plugin.Host, r rule.Rule) error {
	for _, h := range among {
		err := h.Add(r)
		if err == nil || !errors.Is(err, plugin.ErrUnhosted) {
			return err
		}
	}
	return fmt.Errorf("%w: no plugin in use catches the moment of the rule %v", plugin.ErrUnhosted, r)
}
