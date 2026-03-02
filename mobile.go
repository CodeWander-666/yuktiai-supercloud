//go:build android
// +build android

package main

import (
	"log"
)

//export StartNode
func StartNode() {
	go func() {
		log.Println("Starting node from Android")
		// Replace this with your actual node startup logic.
		// For example, call a function that initializes your node.
		runNode()
	}()
}

// runNode is a placeholder – you need to put your actual node logic here.
func runNode() {
	// Copy the essential parts of your main() that start the node.
	// Make sure it doesn't block forever.
}
