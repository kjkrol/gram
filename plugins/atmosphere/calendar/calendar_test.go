package calendar_test

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/world/clock"
)

// A calendar reads the clock at a fixed scale: a fresh game begins at Start in the middle of
// Season, a Day of game time later it is the next day, and the year comes round.
func TestCalendar_ReadsTheClockAtAFixedScale(t *testing.T) {
	c := calendar.New(clock.New(clock.Config{}), calendar.Config{Day: time.Minute, Season: calendar.Autumn})
	if m := c.At(0); m.Date != 5 || m.Time != 8.0/24 || m.Season() != calendar.Autumn || m.Hour() != "08:00" || m.Written() != "day 6 of 8" {
		t.Errorf("a fresh game begins at %+v (%s, %s), want day 6 of 8 at 08:00 in autumn", m, m.Hour(), m.Written())
	}
	if m := c.At(time.Minute + 16*time.Second); m.Date != 6 || m.Hour() != "14:24" {
		t.Errorf("a day and 16 s on it is %+v (%s), want the next day at 14:24", m, m.Hour())
	}
	if m := c.At(3 * time.Minute); m.Date != 0 || m.Season() != calendar.Spring {
		t.Errorf("three days on the year has not come round: %+v", m)
	}
	e := calendar.New(clock.New(clock.Config{}), calendar.Config{Day: time.Minute, Year: calendar.EarthYear, Season: calendar.Spring})
	if m := e.At(0); m.Written() != "4 May" || m.Season() != calendar.Spring {
		t.Errorf("an EarthYear begins in mid-spring on %s in %s, want 4 May in spring", m.Written(), m.Season())
	}
}

// Daily and Yearly give a schedule entry the period and the offset of its first coming.
func TestCalendar_DailyAndYearlyEntries(t *testing.T) {
	c := calendar.New(clock.New(clock.Config{}), calendar.Config{Day: time.Minute})
	if period, offset := c.Daily(22.0 / 24); period != time.Minute || offset != 35*time.Second {
		t.Errorf("every day at 22:00 is every %v from %v, want a minute from 35 s (14 hours of a minute-long day)", period, offset)
	}
	if period, offset := c.Daily(2.0 / 24); period != time.Minute || offset != 45*time.Second {
		t.Errorf("every day at 02:00 is every %v from %v, want a minute from 45 s: the next day's", period, offset)
	}
	period, offset := c.Seasonal(calendar.Winter)
	if period != 8*time.Minute || c.At(offset).Season() != calendar.Winter || c.At(offset-time.Second).Season() != calendar.Autumn {
		t.Errorf("every winter is every %v from %v, want 8 minutes from the first moment of winter", period, offset)
	}
}
