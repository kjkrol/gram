package main

import "github.com/kjkrol/gram/plugins/light"

// a sun of mid-morning, low enough to show the ground's shape
var sun = light.Distant{
	Dir:          [3]float32{0.6, 0.35, 0.72},
	Strength:     0.75,
	Ambient:      0.38,
	Color:        [3]float32{1, 0.96, 0.88},
	AmbientColor: [3]float32{0.82, 0.88, 1},
}
