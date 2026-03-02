//go:build android
// +build android

package main

import (
	"log"
	"net/http"
	"time"
)

//export StartNode
func StartNode() {
	// Run the node in a background goroutine
	go func() {
		log.Println("Starting node from Android")
		// Initialize the node (you'll need to adapt your main() logic)
		// For simplicity, call a function that sets up your node.
		// This should start the libp2p host and the HTTP dashboard.
		runNode()
	}()
}

// runNode contains the core node logic (copied from your main).
func runNode() {
	// Place here the essential parts of your main() function that:
	// - Loads identity
	// - Creates libp2p host, DHT, pubsub
	// - Starts discovery
	// - Starts the dashboard HTTP server
	// Make sure to use a context and avoid blocking forever.
}
