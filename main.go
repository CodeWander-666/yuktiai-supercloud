package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
)

const (
<<<<<<< HEAD
	bootstrapURL = "https://your-domain.com/bootstrap.json" // Replace with your actual URL
=======
	bootstrapURL = "https://your-site.infinityfreeapp.com/bootstrap.json"
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
	serviceTopic = "supercloud-services"
	jobTopic     = "supercloud-jobs"
	keyFile      = "node.key"
)

type Node struct {
	host     host.Host
	dht      *dht.IpfsDHT
	ps       *pubsub.PubSub
	peerID   peer.ID
<<<<<<< HEAD
	services map[string]string
	jobs     map[string]string
=======
	services map[string]string // service ID -> manifest URL
	jobs     map[string]string // job ID -> result (in-memory)
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
	ctx      context.Context
	cancel   context.CancelFunc
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	node := &Node{
		ctx:      ctx,
		cancel:   cancel,
		services: make(map[string]string),
		jobs:     make(map[string]string),
	}

	// Load or create identity
	if err := node.loadIdentity(); err != nil {
		log.Fatal("identity error:", err)
	}

	// Setup libp2p host
	if err := node.setupHost(); err != nil {
		log.Fatal("host error:", err)
	}

	// Setup DHT
	if err := node.setupDHT(); err != nil {
		log.Fatal("dht error:", err)
	}

	// Setup PubSub
	if err := node.setupPubSub(); err != nil {
		log.Fatal("pubsub error:", err)
	}

	// Bootstrap network
	if err := node.bootstrap(); err != nil {
		log.Println("bootstrap warning:", err)
	}

	// Start service discovery
	node.startDiscovery()

<<<<<<< HEAD
	// Start web dashboard
	go node.startDashboard(":8080")

	// Wait for shutdown
=======
	// Start HTTP API for local status (optional)
	go node.startHTTPAPI(":8080")

	// Handle shutdown
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Println("shutting down...")
	node.host.Close()
}

<<<<<<< HEAD
// loadIdentity loads or generates an Ed25519 key
=======
// loadIdentity loads or generates a Ed25519 key
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
func (n *Node) loadIdentity() error {
	if data, err := os.ReadFile(keyFile); err == nil {
		priv, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return err
		}
		n.host, _ = libp2p.New(libp2p.Identity(priv))
		return nil
	}
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return err
	}
	data, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return err
	}
<<<<<<< HEAD
	if err := os.WriteFile(keyFile, data, 0600); err != nil {
		return err
	}
=======
	os.WriteFile(keyFile, data, 0600)
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
	n.host, _ = libp2p.New(libp2p.Identity(priv))
	return nil
}

<<<<<<< HEAD
// setupHost creates the libp2p host
func (n *Node) setupHost() error {
=======
func (n *Node) setupHost() error {
	// Simple host with default transports
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic",
		),
	)
	if err != nil {
		return err
	}
	n.host = h
	n.peerID = h.ID()
	log.Printf("Peer ID: %s", n.peerID)
	for _, addr := range h.Addrs() {
		log.Printf("Listening on: %s", addr)
	}
	return nil
}

<<<<<<< HEAD
// setupDHT creates a Kademlia DHT
func (n *Node) setupDHT() error {
=======
func (n *Node) setupDHT() error {
	// Create a DHT instance with the host
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
	kadDHT, err := dht.New(n.ctx, n.host)
	if err != nil {
		return err
	}
	n.dht = kadDHT
	return nil
}

<<<<<<< HEAD
// setupPubSub creates GossipSub
=======
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
func (n *Node) setupPubSub() error {
	ps, err := pubsub.NewGossipSub(n.ctx, n.host)
	if err != nil {
		return err
	}
	n.ps = ps

<<<<<<< HEAD
=======
	// Subscribe to service and job topics
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
	subService, err := ps.Subscribe(serviceTopic)
	if err != nil {
		return err
	}
	go n.handleServiceMessages(subService)

	subJob, err := ps.Subscribe(jobTopic)
	if err != nil {
		return err
	}
	go n.handleJobMessages(subJob)

	return nil
}

<<<<<<< HEAD
// bootstrap connects to known bootstrap peers from remote URL
func (n *Node) bootstrap() error {
=======
func (n *Node) bootstrap() error {
	// Fetch bootstrap peers from remote URL
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
	peers, err := fetchBootstrapPeers(bootstrapURL)
	if err != nil {
		return err
	}
	for _, pi := range peers {
		if pi.ID == n.peerID {
			continue
		}
		err := n.host.Connect(n.ctx, pi)
		if err != nil {
			log.Printf("failed to connect to %s: %v", pi.ID, err)
		} else {
			log.Printf("connected to bootstrap %s", pi.ID)
		}
	}
	return n.dht.Bootstrap(n.ctx)
}

<<<<<<< HEAD
// fetchBootstrapPeers downloads and parses the bootstrap list
=======
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
func fetchBootstrapPeers(url string) ([]peer.AddrInfo, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var addrs []string
	if err := json.NewDecoder(resp.Body).Decode(&addrs); err != nil {
		return nil, err
	}
	var peers []peer.AddrInfo
	for _, s := range addrs {
		maddr, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			continue
		}
		pi, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			continue
		}
		peers = append(peers, *pi)
	}
	return peers, nil
}

<<<<<<< HEAD
// startDiscovery advertises this node and discovers others
func (n *Node) startDiscovery() {
	routingDiscovery := routing.NewRoutingDiscovery(n.dht)
	util.Advertise(n.ctx, routingDiscovery, "supercloud-nodes")

=======
func (n *Node) startDiscovery() {
	// Advertise ourselves in the DHT
	routingDiscovery := routing.NewRoutingDiscovery(n.dht)
	util.Advertise(n.ctx, routingDiscovery, "supercloud-nodes")

	// Continuously find peers
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
	go func() {
		for {
			peerChan, err := routingDiscovery.FindPeers(n.ctx, "supercloud-nodes")
			if err != nil {
				log.Println("find peers error:", err)
				time.Sleep(time.Minute)
				continue
			}
			for p := range peerChan {
				if p.ID == n.peerID {
					continue
				}
				err := n.host.Connect(n.ctx, p)
				if err != nil {
					log.Printf("failed to connect to discovered peer %s: %v", p.ID, err)
				} else {
					log.Printf("connected to discovered peer %s", p.ID)
				}
			}
			time.Sleep(time.Minute)
		}
	}()
}

<<<<<<< HEAD
// handleServiceMessages listens for new services
=======
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
func (n *Node) handleServiceMessages(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(n.ctx)
		if err != nil {
			log.Println("service sub error:", err)
			return
		}
		var svc struct {
<<<<<<< HEAD
			ID       string `json:"id"`
=======
			ID     string `json:"id"`
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
			Manifest string `json:"manifest"`
		}
		if err := json.Unmarshal(msg.Data, &svc); err != nil {
			continue
		}
		n.services[svc.ID] = svc.Manifest
		log.Printf("new service registered: %s", svc.ID)
	}
}

<<<<<<< HEAD
// handleJobMessages listens for jobs and processes them
=======
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
func (n *Node) handleJobMessages(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(n.ctx)
		if err != nil {
			log.Println("job sub error:", err)
			return
		}
		var job struct {
			ID      string `json:"id"`
			Prompt  string `json:"prompt"`
			Service string `json:"service"`
		}
		if err := json.Unmarshal(msg.Data, &job); err != nil {
			continue
		}
<<<<<<< HEAD
		// For simplicity, every node processes every job (deduplication can be added later)
		log.Printf("processing job %s", job.ID)
		time.Sleep(2 * time.Second) // simulate inference
		result := fmt.Sprintf("Result for: %s", job.Prompt)
		n.jobs[job.ID] = result

		// Publish result
		resMsg, _ := json.Marshal(map[string]string{
			"id":     job.ID,
			"result": result,
		})
		n.ps.Publish(jobTopic, resMsg)
	}
}

// startDashboard serves a simple web UI
func (n *Node) startDashboard(addr string) {
	http.HandleFunc("/", n.dashboardHandler)
	http.HandleFunc("/status", n.statusHandler)
	log.Println("Dashboard available at", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// dashboardHandler renders the HTML dashboard
func (n *Node) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Supercloud Node</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .card { background: white; padding: 20px; margin: 20px 0; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; }
        .info { color: #666; margin: 10px 0; }
        .label { font-weight: bold; color: #444; }
        table { width: 100%; border-collapse: collapse; }
        th, td { text-align: left; padding: 8px; border-bottom: 1px solid #ddd; }
        th { background-color: #f2f2f2; }
    </style>
    <meta http-equiv="refresh" content="5">
</head>
<body>
    <h1>Supercloud Node Dashboard</h1>
    <div class="card">
        <div class="info"><span class="label">Peer ID:</span> {{.PeerID}}</div>
        <div class="info"><span class="label">Addresses:</span> {{.Addresses}}</div>
    </div>
    <div class="card">
        <h2>Registered Services</h2>
        <table>
            <tr><th>Service ID</th><th>Manifest URL</th></tr>
            {{range $id, $manifest := .Services}}
            <tr><td>{{$id}}</td><td>{{$manifest}}</td></tr>
            {{else}}
            <tr><td colspan="2">No services</td></tr>
            {{end}}
        </table>
    </div>
    <div class="card">
        <h2>Recent Jobs</h2>
        <table>
            <tr><th>Job ID</th><th>Result</th></tr>
            {{range $id, $result := .Jobs}}
            <tr><td>{{$id}}</td><td>{{$result}}</td></tr>
            {{else}}
            <tr><td colspan="2">No jobs</td></tr>
            {{end}}
        </table>
    </div>
</body>
</html>`
	t := template.Must(template.New("dashboard").Parse(tmpl))
	data := struct {
		PeerID    string
		Addresses []multiaddr.Multiaddr
		Services  map[string]string
		Jobs      map[string]string
	}{
		PeerID:    n.peerID.String(),
		Addresses: n.host.Addrs(),
		Services:  n.services,
		Jobs:      n.jobs,
	}
	t.Execute(w, data)
}

// statusHandler returns JSON for monitoring
func (n *Node) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peer_id":   n.peerID.String(),
		"addresses": n.host.Addrs(),
		"services":  n.services,
		"jobs":      n.jobs,
	})
}
=======
		// Only process jobs assigned to us (simple hash modulo)
		if shouldProcess(job.ID, n.peerID) {
			log.Printf("processing job %s", job.ID)
			// Simulate inference
			time.Sleep(2 * time.Second)
			result := fmt.Sprintf("Result for: %s", job.Prompt)
			n.jobs[job.ID] = result
			// Publish result
			resMsg, _ := json.Marshal(map[string]string{
				"id":     job.ID,
				"result": result,
			})
			n.ps.Publish(jobTopic, resMsg)
		}
	}
}

func shouldProcess(jobID string, self peer.ID) bool {
	// Simple deterministic assignment: first byte of jobID mod number of peers
	// In a real network, you'd need a consistent view of peers.
	// For now, always process – let all nodes process (duplicate) and deduplicate later.
	// This is simplified for stability.
	return true
}

func (n *Node) startHTTPAPI(addr string) {
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"peer_id":  n.peerID.String(),
			"services": n.services,
		})
	})
	http.HandleFunc("/services", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(n.services)
	})
	log.Println("HTTP API listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
>>>>>>> dd2415e73e5709cd1d39d7e9d747bf898b18e39d
