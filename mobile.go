//go:build android
// +build android

package main

import "C"
import (
	"log"
	"time"
)

//export StartNode
func StartNode() {
	go func() {
		time.Sleep(100 * time.Millisecond)
		log.Println("Starting node from Android")
		main() // or a dedicated function that doesn't block
	}()
}

// Keep the original main() unchanged (your existing main.go)
