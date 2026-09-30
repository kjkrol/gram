package calendar

import (
	"fmt"
	"math"
	"time"

	"github.com/kjkrol/gram/plugins/world/clock"
)

// Year is how the year goes: how many days it has and how long the moon takes round.
type Year uint8

const (
	// GameYear is eight days, two a season, the moon round in four: a year to watch go by.
	GameYear Year = iota
	// EarthYear is 365 days, the months as they are, the moon round in 29.5: the year as it is.
	EarthYear
)

// Days is how many days the year has.
func (y Year) Days() int32 {
	if y == EarthYear {
		return 365
	}
	return 8
}

// Lunation is how many days the moon takes from new to new.
func (y Year) Lunation() float32 {
	if y == EarthYear {
		return 29.53
	}
	return 4
}

// Season is a quarter of the year.
type Season uint8

const (
	Spring Season = iota
	Summer
	Autumn
	Winter
)

func (s Season) String() string {
	return [...]string{"spring", "summer", "autumn", "winter"}[s%4]
}

// Moment is a moment of the calendar: Date the day of the year, 0 the first of spring, Time the
// part of the day gone, 0 at midnight, 0.5 at noon, of a Year.
type Moment struct {
	Date int32
	Time float32
	Year Year
}

// OfYear is the part of the year gone, 0 at the first of spring up to 1.
func (m Moment) OfYear() float32 {
	return (float32(m.Date) + m.Time) / float32(m.Year.Days())
}

// Season is the quarter of the year the moment falls in.
func (m Moment) Season() Season { return Season(min(int(m.OfYear()*4), 3)) }

// Moon is how far the moon is round from new, 0 up to 1: 0.5 full.
func (m Moment) Moon() float32 {
	round := float64(m.Date) + float64(m.Time)
	lunation := float64(m.Year.Lunation())
	return float32(math.Mod(round, lunation) / lunation)
}

// Hour is the time of day as hours and minutes: 08:00.
func (m Moment) Hour() string {
	minutes := int(m.Time*24*60+0.5) % (24 * 60)
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

// Written is the date as a calendar says it: day 3 of 8, or 20 March.
func (m Moment) Written() string {
	if m.Year != EarthYear {
		return fmt.Sprintf("day %d of %d", m.Date+1, m.Year.Days())
	}
	day := (int(m.Date) + firstOfSpring) % 365
	for i, n := range monthDays {
		if day < n {
			return fmt.Sprintf("%d %s", day+1, monthNames[i])
		}
		day -= n
	}
	return ""
}

// MoonName is the moon's phase by name.
func (m Moment) MoonName() string {
	names := [...]string{"new moon", "waxing crescent", "first quarter", "waxing gibbous", "full moon", "waning gibbous", "last quarter", "waning crescent"}
	return names[int(m.Moon()*8+0.5)%8]
}

// The months of an EarthYear, and the day of it, from the first of January, spring begins on.
var (
	monthDays     = [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	monthNames    = [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	firstOfSpring = 78 // 20 March
)

// Config is the calendar: how long a Day is in game time, the time of day a fresh game begins at
// on the clock's face (Start: 23*time.Hour + 30*time.Minute is half past eleven at night; midnight
// is 24*time.Hour), the Year, and the Season a fresh game begins in the middle of. Zero fields are
// DefaultDay, 8 in the morning, a GameYear and spring.
type Config struct {
	Day    time.Duration
	Start  time.Duration
	Year   Year
	Season Season
}

// DefaultDay is how long a day is unless the Config says otherwise: 16 minutes of game time, a day
// going by slowly enough to watch the light turn.
const DefaultDay = 16 * time.Minute

func (c Config) withDefaults() Config {
	if c.Day <= 0 {
		c.Day = DefaultDay
	}
	if c.Start == 0 {
		c.Start = 8 * time.Hour
	}
	return c
}

// begins is the part of the day a fresh game begins at, 0 to 1.
func (c Config) begins() float64 {
	day := float64(24 * time.Hour)
	f := float64(c.Start) / day
	return f - math.Floor(f)
}

// midst is the day in the middle of season.
func (c Config) midst(season Season) int32 {
	days := c.Year.Days()
	return (int32(season%4)*days + days/2) / 4
}

// Calendar turns the tactical clock's game time into a date and a time of day: the clock at a
// fixed scale, a day every Config.Day of game time from the moment a fresh game begins at. It
// keeps nothing of its own — a loaded game's clock brings its date back — and never jumps: the
// day is hurried by the clock's tempo.
type Calendar struct {
	cfg   Config
	clock *clock.Clock
}

// New is a calendar of cfg over clk.
func New(clk *clock.Clock, cfg Config) *Calendar {
	return &Calendar{cfg: cfg.withDefaults(), clock: clk}
}

// Config is the calendar's, with its defaults filled in.
func (c *Calendar) Config() Config { return c.cfg }

// Now is the moment the clock stands at.
func (c *Calendar) Now() Moment { return c.At(c.clock.Time()) }

// At is the moment the clock stands at when it reads t.
func (c *Calendar) At(t time.Duration) Moment {
	days := float64(c.cfg.midst(c.cfg.Season)) + c.cfg.begins() + t.Seconds()/c.cfg.Day.Seconds()
	whole := math.Floor(days)
	year := int64(c.cfg.Year.Days())
	return Moment{Date: int32(((int64(whole) % year) + year) % year), Time: float32(days - whole), Year: c.cfg.Year}
}

// Daily is the period and the offset of every day at hour (0 to 1 of the day), for clock.Every:
// clock.Every(cal.Daily(22.0 / 24)).
func (c *Calendar) Daily(hour float32) (period, offset time.Duration) {
	first := float64(hour) - c.cfg.begins()
	first -= math.Floor(first)
	return c.cfg.Day, time.Duration(first * float64(c.cfg.Day)).Round(time.Millisecond)
}

// Yearly is the period and the offset of every year at ofYear (0 to 1 of the year, 0 the first of
// spring), for clock.Every: every winter is Yearly(0.75).
func (c *Calendar) Yearly(ofYear float32) (period, offset time.Duration) {
	days := float64(c.cfg.Year.Days())
	year := time.Duration(days * float64(c.cfg.Day))
	begun := (float64(c.cfg.midst(c.cfg.Season)) + c.cfg.begins()) / days
	first := float64(ofYear) - begun
	first -= math.Floor(first)
	return year, time.Duration(first * float64(year)).Round(time.Millisecond)
}

// Seasonal is Yearly at the start of season.
func (c *Calendar) Seasonal(season Season) (period, offset time.Duration) {
	return c.Yearly(float32(season%4) / 4)
}
