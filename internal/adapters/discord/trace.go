package discord

import (
	"log"
	"time"
)

func step(label string) func() {
	start := time.Now()
	return func() { log.Printf("[trace] %s = %s", label, time.Since(start)) }
}
