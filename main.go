package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/libp2p/go-libp2p/p2p/transport/websocket"
	"github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/host/autonat"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p-record"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"go.uber.org/zap"
)

const (
	BootstrapURL   = "https://your-site.infinityfreeapp.com/bootstrap.json"
	ProtocolID     = protocol.ID("/supercloud/1.0.0")
	ServiceTopic   = "supercloud-services"
	JobTopic       = "supercloud-jobs"
	VerificationTopic = "supercloud-verifications"
	ReputationTopic   = "supercloud-reputation"
	KeyFile        = "node.key" // stored locally
)

type Node struct {
	host       host.Host
	dht        *dual.DHT
	ps         *pubsub.PubSub
	topics     map[string]*pubsub.Topic
	subs       map[string]*pubsub.Subscription
	peerID     peer.ID
	privKey    crypto.PrivKey
	services   map[string]*Service
	jobQueue   map[string]*Job
	reputation map[peer.ID]float64
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *zap.Logger
	httpServer *http.Server
}

type Service struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	ModelURL    string   `json:"model_url"`
	Price       float64  `json:"price"`
	Owner       peer.ID  `json:"owner"`
	Replicas    int      `json:"replicas"`
	Manifest    string   `json:"manifest"` // IPFS hash or URL
	CreatedAt   time.Time `json:"created_at"`
}

type Job struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Prompt    string    `json:"prompt"`
	Result    string    `json:"result"`
	Status    string    `json:"status"`
	NodeID    peer.ID   `json:"node_id"`
	CreatedAt time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at"`
	Tokens    int       `json:"tokens"`
}

type Verification struct {
	JobID     string    `json:"job_id"`
	Verifier  peer.ID   `json:"verifier"`
	Result    string    `json:"result"`
	Similarity float64  `json:"similarity"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup logging
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	node := &Node{
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		services:   make(map[string]*Service),
		jobQueue:   make(map[string]*Job),
		reputation: make(map[peer.ID]float64),
		topics:     make(map[string]*pubsub.Topic),
		subs:       make(map[string]*pubsub.Subscription),
	}

	// Load or generate private key
	if err := node.loadOrCreateKey(); err != nil {
		logger.Fatal("failed to load key", zap.Error(err))
	}

	// Setup libp2p host
	if err := node.setupHost(); err != nil {
		logger.Fatal("failed to setup host", zap.Error(err))
	}

	// Setup DHT
	if err := node.setupDHT(); err != nil {
		logger.Fatal("failed to setup DHT", zap.Error(err))
	}

	// Setup PubSub
	if err := node.setupPubSub(); err != nil {
		logger.Fatal("failed to setup pubsub", zap.Error(err))
	}

	// Bootstrap network
	if err := node.bootstrap(); err != nil {
		logger.Warn("bootstrap failed", zap.Error(err))
	}

	// Start service discovery
	node.startDiscovery()

	// Start HTTP API (optional, for local status)
	node.startHTTPAPI(":8080")

	// Wait for shutdown signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	logger.Info("shutting down...")
	node.httpServer.Shutdown(ctx)
	node.host.Close()
}

func (n *Node) loadOrCreateKey() error {
	// Try to load existing key
	if data, err := os.ReadFile(KeyFile); err == nil {
		n.privKey, err = crypto.UnmarshalPrivateKey(data)
		return err
	}
	// Generate new key
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return err
	}
	n.privKey = priv
	data, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return err
	}
	return os.WriteFile(KeyFile, data, 0600)
}

func (n *Node) setupHost() error {
	cm, err := connmgr.NewConnManager(
		100, // low
		200, // high
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		return err
	}

	// Enable autorelay and autonat
	relayOpt := libp2p.EnableAutoRelay()
	natOpt := libp2p.EnableNATService()

	// Create host
	h, err := libp2p.New(
		libp2p.Identity(n.privKey),
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic",
			"/ip6/::/tcp/0",
			"/ip6/::/udp/0/quic",
			"/ip4/0.0.0.0/tcp/0/ws", // websocket for browsers
		),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(websocket.New),
		libp2p.Transport(quic.NewTransport),
		libp2p.DefaultTransports,
		libp2p.ConnectionManager(cm),
		libp2p.NATPortMap(),
		relayOpt,
		natOpt,
		libp2p.EnableHolePunching(),
		libp2p.UserAgent("supercloud/1.0"),
	)
	if err != nil {
		return err
	}
	n.host = h
	n.peerID = h.ID()
	n.logger.Info("host created", zap.String("peer", n.peerID.String()))
	for _, addr := range h.Addrs() {
		n.logger.Info("listening on", zap.String("addr", addr.String()))
	}
	return nil
}

func (n *Node) setupDHT() error {
	// Create DHT with dual mode (WAN/LAN)
	d, err := dual.New(n.ctx, n.host, dual.DHTOption(
		dht.Mode(dht.ModeAuto),
		dht.ProtocolPrefix(protocol.ID("/supercloud")),
		dht.ProtocolExtension(protocol.ID("/dht")),
	))
	if err != nil {
		return err
	}
	n.dht = d
	return nil
}

func (n *Node) setupPubSub() error {
	ps, err := pubsub.NewGossipSub(n.ctx, n.host,
		pubsub.WithPeerExchange(true),
		pubsub.WithFloodPublish(true),
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
	)
	if err != nil {
		return err
	}
	n.ps = ps

	// Subscribe to topics
	topics := []string{ServiceTopic, JobTopic, VerificationTopic, ReputationTopic}
	for _, t := range topics {
		topic, err := ps.Join(t)
		if err != nil {
			return err
		}
		n.topics[t] = topic
		sub, err := topic.Subscribe()
		if err != nil {
			return err
		}
		n.subs[t] = sub
		go n.handlePubSub(t, sub)
	}
	return nil
}

func (n *Node) bootstrap() error {
	// Fetch bootstrap peers from remote URL
	peers, err := fetchBootstrapPeers(BootstrapURL)
	if err != nil {
		return err
	}
	for _, pi := range peers {
		if pi.ID == n.peerID {
			continue
		}
		n.host.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.PermanentAddrTTL)
		err := n.host.Connect(n.ctx, pi)
		if err != nil {
			n.logger.Warn("failed to connect to bootstrap", zap.String("peer", pi.ID.String()), zap.Error(err))
		} else {
			n.logger.Info("connected to bootstrap", zap.String("peer", pi.ID.String()))
		}
	}
	// Bootstrap DHT
	return n.dht.Bootstrap(n.ctx)
}

func fetchBootstrapPeers(url string) ([]peer.AddrInfo, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var addrs []string
	if err := json.Unmarshal(body, &addrs); err != nil {
		return nil, err
	}
	var peers []peer.AddrInfo
	for _, addrStr := range addrs {
		maddr, err := multiaddr.NewMultiaddr(addrStr)
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
	// Advertise ourselves
	routingDiscovery := routing.NewRoutingDiscovery(n.dht)
	util.Advertise(n.ctx, routingDiscovery, "supercloud-nodes")

	// Find peers continuously
	go func() {
		for {
			peerChan, err := routingDiscovery.FindPeers(n.ctx, "supercloud-nodes")
			if err != nil {
				n.logger.Warn("find peers error", zap.Error(err))
				time.Sleep(time.Minute)
				continue
			}
			for p := range peerChan {
				if p.ID == n.peerID {
					continue
				}
				n.host.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.TempAddrTTL)
				err := n.host.Connect(n.ctx, p)
				if err != nil {
					n.logger.Debug("connect to discovered peer failed", zap.String("peer", p.ID.String()), zap.Error(err))
				} else {
					n.logger.Debug("connected to discovered peer", zap.String("peer", p.ID.String()))
				}
			}
			time.Sleep(time.Minute)
		}
	}()
}

func (n *Node) handlePubSub(topic string, sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(n.ctx)
		if err != nil {
			n.logger.Error("pubsub error", zap.String("topic", topic), zap.Error(err))
			return
		}
		// Ignore self messages
		if msg.ReceivedFrom == n.host.ID() {
			continue
		}
		switch topic {
		case ServiceTopic:
			n.handleServiceMessage(msg)
		case JobTopic:
			n.handleJobMessage(msg)
		case VerificationTopic:
			n.handleVerificationMessage(msg)
		case ReputationTopic:
			n.handleReputationMessage(msg)
		}
	}
}

func (n *Node) handleServiceMessage(msg *pubsub.Message) {
	var svc Service
	if err := json.Unmarshal(msg.Data, &svc); err != nil {
		n.logger.Error("invalid service message", zap.Error(err))
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	// Store in memory and optionally in DHT
	n.services[svc.ID] = &svc
	n.logger.Info("service registered", zap.String("id", svc.ID), zap.String("name", svc.Name))
	// Also store in DHT for persistence
	key := "/services/" + svc.ID
	if err := n.dht.PutValue(n.ctx, key, msg.Data); err != nil {
		n.logger.Error("failed to store service in DHT", zap.Error(err))
	}
}

func (n *Node) handleJobMessage(msg *pubsub.Message) {
	var job Job
	if err := json.Unmarshal(msg.Data, &job); err != nil {
		n.logger.Error("invalid job message", zap.Error(err))
		return
	}
	// Check if this job is assigned to us (consistent hashing)
	if shouldHandleJob(job.ID, n.peerID) {
		n.mu.Lock()
		n.jobQueue[job.ID] = &job
		n.mu.Unlock()
		go n.executeJob(&job)
	}
}

func shouldHandleJob(jobID string, self peer.ID) bool {
	// Consistent hashing: hash jobID and see if this node is responsible
	// Simplified: use modulo on number of nodes (but need global view)
	// For now, just accept all jobs that come to us via gossip
	return true
}

func (n *Node) executeJob(job *Job) {
	n.logger.Info("executing job", zap.String("id", job.ID))
	// Placeholder for model inference
	time.Sleep(2 * time.Second)
	job.Result = "Echo: " + job.Prompt
	job.Status = "completed"
	job.Tokens = 10
	job.CompletedAt = time.Now()
	n.mu.Lock()
	n.jobQueue[job.ID] = job
	n.mu.Unlock()

	// Publish result
	data, _ := json.Marshal(job)
	n.topics[JobTopic].Publish(n.ctx, data)

	// Trigger verification
	n.requestVerification(job)
}

func (n *Node) requestVerification(job *Job) {
	// Randomly select a few peers to verify (simplified: just send to all)
	// In practice, choose based on reputation
	verif := Verification{
		JobID:     job.ID,
		Verifier:  n.peerID,
		Result:    job.Result,
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(verif)
	n.topics[VerificationTopic].Publish(n.ctx, data)
}

func (n *Node) handleVerificationMessage(msg *pubsub.Message) {
	var verif Verification
	if err := json.Unmarshal(msg.Data, &verif); err != nil {
		return
	}
	// If we are the original node, ignore
	if verif.Verifier == n.peerID {
		return
	}
	// Run the same job again to verify
	job, ok := n.jobQueue[verif.JobID]
	if !ok {
		return
	}
	// Simulate re-execution
	time.Sleep(1 * time.Second)
	reResult := "Echo: " + job.Prompt
	similarity := 1.0
	if reResult != verif.Result {
		similarity = 0.0
	}
	// Update reputation based on similarity
	n.updateReputation(verif.Verifier, similarity)
}

func (n *Node) updateReputation(peerID peer.ID, score float64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	current := n.reputation[peerID]
	if score > 0.9 {
		current += 0.01
	} else {
		current -= 0.05
	}
	if current < 0 {
		current = 0
	}
	if current > 1 {
		current = 1
	}
	n.reputation[peerID] = current
	n.logger.Info("reputation updated", zap.String("peer", peerID.String()), zap.Float64("score", current))
}

func (n *Node) handleReputationMessage(msg *pubsub.Message) {
	// Gossiped reputation updates (optional)
	var rep struct {
		Peer  peer.ID `json:"peer"`
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal(msg.Data, &rep); err != nil {
		return
	}
	n.mu.Lock()
	n.reputation[rep.Peer] = rep.Score
	n.mu.Unlock()
}

func (n *Node) startHTTPAPI(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"peer_id": n.peerID.String(),
			"addresses": n.host.Addrs(),
			"services": n.services,
			"reputation": n.reputation,
		})
	})
	mux.HandleFunc("/services", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(n.services)
	})
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		n.mu.RLock()
		defer n.mu.RUnlock()
		json.NewEncoder(w).Encode(n.jobQueue)
	})
	n.httpServer = &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := n.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			n.logger.Error("http server error", zap.Error(err))
		}
	}()
	n.logger.Info("HTTP API listening", zap.String("addr", addr))
}
