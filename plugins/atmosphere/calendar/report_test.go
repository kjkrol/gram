package calendar_test

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
)

func TestReporter_SaysTheSeasonAndTheDateALineEach(t *testing.T) {
	c := calendar.New(clock.New(clock.Config{}), calendar.Config{
		DayRealTime: time.Minute,
		Year:        calendar.EarthYear,
		Season:      calendar.Spring,
	})
	got := map[string]string{}
	c.Reporter().Report(func(label, value string) { got[label] = value })
	if got["Time of day"] != "08:00" || got["Season"] != "spring\n4 May" || got["Moon"] == "" {
		t.Errorf("the calendar says %q, want 08:00, spring and 4 May two lines, and the moon", got)
	}
}
