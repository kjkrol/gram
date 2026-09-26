package climate_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/climate"
	"github.com/kjkrol/gram/plugins/sky"
)

// midwinter and midsummer are a zone's coldest and warmest days' mean.
func midwinter(p climate.Profile) float32 { return p.Mean - p.Year }
func midsummer(p climate.Profile) float32 { return p.Mean + p.Year }

func TestZone_TheEquatorIsHotAllYearAndATemperateZoneFreezesInWinter(t *testing.T) {
	eq := climate.Equatorial.Profile()
	if eq.Mean < 25 || eq.Year > 2 {
		t.Errorf("the equator's climate is %+v, want hot and without seasons to speak of", eq)
	}
	temperate := climate.Temperate.Profile()
	if w, s := midwinter(temperate), midsummer(temperate); w > -3 || w < -9 || s < 15 || s > 20 {
		t.Errorf("a temperate midwinter is %v° and midsummer %v°, want about −6 and +17: before the warming", w, s)
	}
}

func TestZone_GrowsColderFromTheEquatorToThePole(t *testing.T) {
	zones := []climate.Zone{climate.Equatorial, climate.Tropical, climate.Mediterranean, climate.Temperate, climate.Cold, climate.Polar}
	for i := 1; i < len(zones); i++ {
		if a, b := zones[i-1].Profile(), zones[i].Profile(); b.Mean >= a.Mean || midwinter(b) >= midwinter(a) {
			t.Errorf("zone %d (%v°) is %v° a year, %v° in winter: not colder than the one before (%v°, %v°)",
				i, zones[i].Latitude, b.Mean, midwinter(b), a.Mean, midwinter(a))
		}
	}
	if polar := climate.Polar.Profile(); midwinter(polar) > -25 || midsummer(polar) > 5 {
		t.Errorf("the polar winter is %v° and its summer %v°, want bitter and barely thawing", midwinter(polar), midsummer(polar))
	}
}

func TestZone_FactorsShapeWhatTheLatitudeGave(t *testing.T) {
	med := climate.Mediterranean.Profile()
	if med.Wet[sky.Summer] >= med.Wet[sky.Winter]/3 {
		t.Errorf("the Mediterranean's summer is %v wet, its winter %v: want the summer dry", med.Wet[sky.Summer], med.Wet[sky.Winter])
	}
	bare := climate.Zone{Latitude: 55}.Profile()
	warmed := climate.Zone{Latitude: 55, Factors: []climate.Factor{climate.SeaCurrent{Warmth: 4}}}.Profile()
	if warmed.Mean != bare.Mean+4 || warmed.Year >= bare.Year {
		t.Errorf("a warm current makes %+v of %+v, want 4° warmer and milder seasons", warmed, bare)
	}
}
