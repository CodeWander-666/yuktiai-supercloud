package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func main() {
	// Get database connection string from environment
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN environment variable not set")
	}
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)

	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/heartbeat", heartbeatHandler)
	http.HandleFunc("/nodes", nodesHandler)
	http.HandleFunc("/job", createJobHandler)
	http.HandleFunc("/assign", assignJobHandler)
	http.HandleFunc("/result", resultHandler)
	http.HandleFunc("/result/", getResultHandler)

	log.Println("Orchestrator starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type RegisterRequest struct {
	NodeID string `json:"node_id"`
	PeerID string `json:"peer_id"`
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := db.Exec(
		"INSERT INTO nodes (id, peer_id, last_heartbeat, status) VALUES (?, ?, NOW(), 'active') "+
			"ON DUPLICATE KEY UPDATE peer_id=?, last_heartbeat=NOW(), status='active'",
		req.NodeID, req.PeerID, req.PeerID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
}

type HeartbeatRequest struct {
	NodeID string `json:"node_id"`
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := db.Exec("UPDATE nodes SET last_heartbeat=NOW() WHERE id=?", req.NodeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func nodesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, peer_id, last_heartbeat, status FROM nodes WHERE status='active' AND last_heartbeat > NOW() - INTERVAL 2 MINUTE")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var nodes []map[string]interface{}
	for rows.Next() {
		var id, peerID, status string
		var lastHeartbeat time.Time
		rows.Scan(&id, &peerID, &lastHeartbeat, &status)
		nodes = append(nodes, map[string]interface{}{
			"id":             id,
			"peer_id":        peerID,
			"last_heartbeat": lastHeartbeat,
			"status":         status,
		})
	}
	json.NewEncoder(w).Encode(nodes)
}

type JobRequest struct {
	Prompt string `json:"prompt"`
}

func createJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Find an active node
	var nodeID string
	err := db.QueryRow("SELECT id FROM nodes WHERE status='active' AND last_heartbeat > NOW() - INTERVAL 2 MINUTE ORDER BY RAND() LIMIT 1").Scan(&nodeID)
	if err != nil {
		http.Error(w, "No active nodes", http.StatusServiceUnavailable)
		return
	}
	jobID := generateJobID()
	_, err = db.Exec("INSERT INTO jobs (id, node_id, prompt, status) VALUES (?, ?, ?, 'pending')", jobID, nodeID, req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"job_id": jobID})
}

func generateJobID() string {
	return "job-" + time.Now().Format("20060102150405") + "-" + randomString(6)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func assignJobHandler(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		http.Error(w, "missing node_id", http.StatusBadRequest)
		return
	}
	var jobID, prompt string
	err := db.QueryRow("SELECT id, prompt FROM jobs WHERE node_id=? AND status='pending' ORDER BY created_at LIMIT 1", nodeID).Scan(&jobID, &prompt)
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNoContent)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Mark as assigned
	db.Exec("UPDATE jobs SET status='assigned' WHERE id=?", jobID)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id": jobID,
		"prompt": prompt,
	})
}

type ResultRequest struct {
	JobID  string `json:"job_id"`
	Result string `json:"result"`
	Tokens int    `json:"tokens"`
}

func resultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := db.Exec("UPDATE jobs SET status='completed', result=? WHERE id=?", req.Result, req.JobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func getResultHandler(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Path[len("/result/"):]
	var result string
	err := db.QueryRow("SELECT result FROM jobs WHERE id=?", jobID).Scan(&result)
	if err == sql.ErrNoRows {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"result": result})
}
