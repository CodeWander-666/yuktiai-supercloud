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
	bootstrapURL = "http://yukti-ai-supercloud.free.nf/bootstrap/bootstrap.json" // Replace with your actual URL
	serviceTopic = "supercloud-services"
	jobTopic     = "supercloud-jobs"
	keyFile      = "node.key"
)

type Node struct {
	host     host.Host
	dht      *dht.IpfsDHT
	ps       *pubsub.PubSub
	peerID   peer.ID
	services map[string]string
	jobs     map[string]string
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

	// Start web dashboard
	go node.startDashboard(":8080")

	// Wait for shutdown
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Println("shutting down...")
	node.host.Close()
}

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
	if err := os.WriteFile(keyFile, data, 0600); err != nil {
		return err
	}
	n.host, _ = libp2p.New(libp2p.Identity(priv))
	return nil
}

func (n *Node) setupHost() error {
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

func (n *Node) setupDHT() error {
	kadDHT, err := dht.New(n.ctx, n.host)
	if err != nil {
		return err
	}
	n.dht = kadDHT
	return nil
}

func (n *Node) setupPubSub() error {
	ps, err := pubsub.NewGossipSub(n.ctx, n.host)
	if err != nil {
		return err
	}
	n.ps = ps

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

func (n *Node) bootstrap() error {
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

func (n *Node) startDiscovery() {
	routingDiscovery := routing.NewRoutingDiscovery(n.dht)
	util.Advertise(n.ctx, routingDiscovery, "supercloud-nodes")

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

func (n *Node) handleServiceMessages(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(n.ctx)
		if err != nil {
			log.Println("service sub error:", err)
			return
		}
		var svc struct {
			ID       string `json:"id"`
			Manifest string `json:"manifest"`
		}
		if err := json.Unmarshal(msg.Data, &svc); err != nil {
			continue
		}
		n.services[svc.ID] = svc.Manifest
		log.Printf("new service registered: %s", svc.ID)
	}
}

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
		log.Printf("processing job %s", job.ID)
		time.Sleep(2 * time.Second)
		result := fmt.Sprintf("Result for: %s", job.Prompt)
		n.jobs[job.ID] = result

		resMsg, _ := json.Marshal(map[string]string{
			"id":     job.ID,
			"result": result,
		})
		n.ps.Publish(jobTopic, resMsg)
	}
}

func (n *Node) startDashboard(addr string) {
	http.HandleFunc("/", n.dashboardHandler)
	http.HandleFunc("/status", n.statusHandler)
	log.Println("Dashboard available at", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

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

func (n *Node) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peer_id":   n.peerID.String(),
		"addresses": n.host.Addrs(),
		"services":  n.services,
		"jobs":      n.jobs,
	})
}
