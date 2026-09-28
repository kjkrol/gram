package main

import (
	"log"
	"os"
	"runtime/pprof"

	"github.com/kjkrol/gram"
)

func main() {
	// TEMP-MEASURE: GRAM_CPUPROFILE=<file> profiles the CPU until the game quits.
	if path := os.Getenv("GRAM_CPUPROFILE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}
	gram.Run(NewDemo())
}
