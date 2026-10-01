package hooks

import (
	"fmt"
	"io"
	"os"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world/rule"
)

// LogFormatter renders one contact as a log line.
type LogFormatter func(m collision.Meeting) string

func defaultLogFormat(m collision.Meeting) string {
	return fmt.Sprintf("collision: %v <-> %v (impact %.2f)", m.Self, m.Other, m.Impact)
}

type logConfig struct {
	w      io.Writer
	format LogFormatter
}

// LogOption adjusts where and how LogContacts writes.
type LogOption func(*logConfig)

// LogTo sends the lines to w instead of stdout.
func LogTo(w io.Writer) LogOption { return func(c *logConfig) { c.w = w } }

// LogAs renders each line with f instead of the default format.
func LogAs(f LogFormatter) LogOption { return func(c *logConfig) { c.format = f } }

// LogContacts writes a line per contact, to stdout in the default format unless told otherwise.
func LogContacts(opts ...LogOption) plugin.Rule {
	c := &logConfig{w: os.Stdout, format: defaultLogFormat}
	for _, opt := range opts {
		opt(c)
	}
	return rule.On("collision.log contacts", rule.All, func(m *rule.Moment[collision.Meeting]) rule.Step {
		return m.Call(func(_ plugin.Tick, met collision.Meeting) { fmt.Fprintln(c.w, c.format(met)) })
	})
}
