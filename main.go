package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
)

func main() {
	// Create a random key pair for this host
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		panic(err)
	}

	// Create a connection manager
	cm, err := connmgr.NewConnManager(
		100, // Lowwater
		200, // Highwater
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		panic(err)
	}

	// Build the host
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",      // TCP
			"/ip4/0.0.0.0/udp/0/quic", // QUIC
		),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.DefaultTransports,
		libp2p.ConnectionManager(cm),
		libp2p.NATPortMap(),
	)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// Print the host's multiaddresses
	fmt.Printf("Host ID: %s\n", h.ID())
	for _, addr := range h.Addrs() {
		fmt.Printf("  %s\n", addr)
	}

	// Set up mDNS discovery (local network)
	mdnsService := mdns.NewMdnsService(h, "supercloud", &discoveryNotifee{h: h})
	if err := mdnsService.Start(); err != nil {
		panic(err)
	}

	fmt.Println("Node started. Press Ctrl-C to stop.")

	// Wait for interrupt
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("Received interrupt, shutting down...")
}

type discoveryNotifee struct {
	h host.Host
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("discovered new peer: %s\n", pi.ID)
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("failed to connect to peer %s: %s\n", pi.ID, err)
	}
}
