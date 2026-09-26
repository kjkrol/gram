package sky

import (
	"fmt"
	"math"
	"time"
)

// Day is the time of day and of the year, the one fact of the sky, held by the sky's own entity
// and saved with it: Time is the part of the day gone, 0 at midnight, 0.5 at noon; Pace how many
// times faster than Length the day goes by; Stopped holds it where it is, the pace kept for when
// it goes on; Date is the day of the year, 0 the first of spring, of the Calendar's days; Length
// is how long a whole day takes at pace 1.
type Day struct {
	Time     float32
	Pace     float32
	Stopped  bool
	Date     int32
	Calendar Calendar
	Length   time.Duration
}

// Calendar is how the year goes: how many days it has and how long the moon takes round.
type Calendar uint8

const (
	// GameYear is eight days, two a season, the moon round in four: a year to watch go by.
	GameYear Calendar = iota
	// EarthYear is 365 days in twelve months, the moon round in 29.5: the year as it is.
	EarthYear
)

// Days is how many days the year has.
func (c Calendar) Days() int32 {
	if c == EarthYear {
		return 365
	}
	return 8
}

// Lunation is how many days the moon takes from new to new.
func (c Calendar) Lunation() float32 {
	if c == EarthYear {
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

// OfYear is the part of the year gone, 0 at the first of spring up to 1.
func (d Day) OfYear() float32 {
	return (float32(d.Date) + d.Time) / float32(d.Calendar.Days())
}

// Season is the quarter of the year the day falls in.
func (d Day) Season() Season { return Season(min(int(d.OfYear()*4), 3)) }

// Moon is how far the moon is round from new, 0 up to 1: 0.5 full.
func (d Day) Moon() float32 {
	round := float64(d.Date) + float64(d.Time)
	lunation := float64(d.Calendar.Lunation())
	return float32(math.Mod(round, lunation) / lunation)
}

// Written is the date as a calendar says it: day 3 of 8, or 20 March.
func (d Day) Written() string {
	if d.Calendar != EarthYear {
		return fmt.Sprintf("day %d of %d", d.Date+1, d.Calendar.Days())
	}
	day := (int(d.Date) + firstOfSpring) % 365
	for m, n := range monthDays {
		if day < n {
			return fmt.Sprintf("%d %s", day+1, monthNames[m])
		}
		day -= n
	}
	return ""
}

// The months of an EarthYear, and the day of it, from the first of January, spring begins on.
var (
	monthDays     = [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	monthNames    = [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	firstOfSpring = 78 // 20 March
)

// passing moves the day on by, a part of a day, into the next days of the year or back into the
// last ones.
func (d *Day) passing(by float32) {
	t := d.Time + by
	days := float32(math.Floor(float64(t)))
	d.Time = t - days
	year := int64(d.Calendar.Days())
	d.Date = int32(((int64(d.Date)+int64(days))%year + year) % year)
}

// Way is where along the ground the sun stands at noon.
type Way uint8

const (
	NorthWest Way = iota
	North
	NorthEast
	East
	SouthEast
	South
	SouthWest
	West
)

// angle is the way as radians from +x (east), y running south.
func (w Way) angle() float64 {
	return [...]float64{-3 * math.Pi / 4, -math.Pi / 2, -math.Pi / 4, 0, math.Pi / 4, math.Pi / 2, 3 * math.Pi / 4, math.Pi}[w%8]
}

// Config is the day and the year: how long a whole day takes at pace 1, the time a Stage starting
// fresh begins at, which way the sun stands at noon, in how many steps a day the sun moves — the
// terrain's shadows are worked out anew at every step — the Calendar, and the Season a fresh Stage
// begins in the middle of. Zero fields are 4 minutes, 8 in the morning, the north-west, 96 steps,
// a GameYear and spring. How high the sun goes is the latitude's (Plugin.SetLatitude).
type Config struct {
	Length   time.Duration
	Start    float32
	NoonWay  Way
	Steps    int
	Calendar Calendar
	Season   Season

	latitude float64 // degrees from the equator, the world's; set by Plugin.SetLatitude
}

// Latitude is where the sky stands without a climate to say otherwise: 30° from the equator, the
// sun 60° up at noon at the equinoxes.
const Latitude = 30

// midst is the day in the middle of season.
func (c Config) midst(season Season) int32 {
	days := c.Calendar.Days()
	return (int32(season%4)*days + days/2) / 4
}

func (c Config) withDefaults() Config {
	if c.Length <= 0 {
		c.Length = 4 * time.Minute
	}
	if c.Start == 0 {
		c.Start = 8.0 / 24
	}
	if c.Steps <= 0 {
		c.Steps = 96
	}
	return c
}
