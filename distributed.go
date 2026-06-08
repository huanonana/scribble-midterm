package scribble

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	operationPath = "/scribble/v1/operations"
	snapshotPath  = "/scribble/v1/snapshot"
)

// Operation is a write or delete command replicated between Scribble nodes.
type Operation struct {
	Type       string          `json:"type"`
	NodeID     string          `json:"node_id,omitempty"`
	Collection string          `json:"collection"`
	Resource   string          `json:"resource,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// SnapshotRecord is a record included in a point-in-time node snapshot.
type SnapshotRecord struct {
	Collection string          `json:"collection"`
	Resource   string          `json:"resource"`
	Data       json.RawMessage `json:"data"`
}

// Snapshot contains all records currently stored by a node.
type Snapshot struct {
	NodeID  string           `json:"node_id,omitempty"`
	Records []SnapshotRecord `json:"records"`
}

func (d *Driver) replicateWrite(collection, resource string, v interface{}) error {
	if len(d.peers) == 0 {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return d.replicate(Operation{
		Type: "write", NodeID: d.nodeID, Collection: collection, Resource: resource, Data: data,
	})
}

func (d *Driver) replicateDelete(collection, resource string) error {
	if len(d.peers) == 0 {
		return nil
	}
	return d.replicate(Operation{
		Type: "delete", NodeID: d.nodeID, Collection: collection, Resource: resource,
	})
}

func (d *Driver) replicate(op Operation) error {
	var failures []string
	for _, peer := range d.peers {
		if err := d.sendOperation(peer, op); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("replication failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (d *Driver) sendOperation(peer string, op Operation) error {
	body, err := json.Marshal(op)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(peer, "/")+operationPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := d.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", peer, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		message, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("%s: status %d: %s", peer, res.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}

// HTTPHandler exposes endpoints used by peer nodes for replication and snapshot sync.
func (d *Driver) HTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(operationPath, d.handleOperation)
	mux.HandleFunc(snapshotPath, d.handleSnapshot)
	return mux
}

func (d *Driver) handleOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var op Operation
	if err := json.NewDecoder(io.LimitReader(r.Body, 10<<20)).Decode(&op); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := d.applyOperation(op); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Driver) applyOperation(op Operation) error {
	switch op.Type {
	case "write":
		if op.Collection == "" || op.Resource == "" || len(op.Data) == 0 {
			return errorsForOperation(op)
		}
		mutex := d.getOrCreateMutex(op.Collection)
		mutex.Lock()
		defer mutex.Unlock()
		dir := filepath.Join(d.dir, op.Collection)
		return write(dir, filepath.Join(dir, op.Resource+".json.tmp"), filepath.Join(dir, op.Resource+".json"), op.Data)
	case "delete":
		if op.Collection == "" {
			return errorsForOperation(op)
		}
		return d.deleteLocal(op.Collection, op.Resource)
	default:
		return fmt.Errorf("unknown operation type %q", op.Type)
	}
}

func errorsForOperation(op Operation) error {
	return fmt.Errorf("invalid %s operation: collection and required fields must be set", op.Type)
}

func (d *Driver) deleteLocal(collection, resource string) error {
	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()
	path := filepath.Join(d.dir, collection, resource)
	fi, err := stat(path)
	if err != nil || fi == nil {
		return fmt.Errorf("unable to find file or directory named %v", filepath.Join(collection, resource))
	}
	if fi.Mode().IsDir() {
		return os.RemoveAll(path)
	}
	return os.RemoveAll(path + ".json")
}

func (d *Driver) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	snapshot, err := d.CreateSnapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snapshot)
}

// CreateSnapshot returns a consistent-enough point-in-time copy for bootstrap and recovery.
func (d *Driver) CreateSnapshot() (Snapshot, error) {
	result := Snapshot{NodeID: d.nodeID, Records: []SnapshotRecord{}}
	err := filepath.Walk(d.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		relative, err := filepath.Rel(d.dir, path)
		if err != nil {
			return err
		}
		parts := strings.Split(relative, string(os.PathSeparator))
		if len(parts) != 2 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result.Records = append(result.Records, SnapshotRecord{
			Collection: parts[0], Resource: strings.TrimSuffix(parts[1], ".json"), Data: data,
		})
		return nil
	})
	return result, err
}

// SyncFrom bootstraps or repairs a node by merging records from a peer snapshot.
func (d *Driver) SyncFrom(peer string) error {
	res, err := d.httpClient().Get(strings.TrimRight(peer, "/") + snapshotPath)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("snapshot request returned status %d", res.StatusCode)
	}
	var snapshot Snapshot
	if err := json.NewDecoder(io.LimitReader(res.Body, 100<<20)).Decode(&snapshot); err != nil {
		return err
	}
	for _, record := range snapshot.Records {
		if err := d.applyOperation(Operation{
			Type: "write", NodeID: snapshot.NodeID, Collection: record.Collection, Resource: record.Resource, Data: record.Data,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) httpClient() *http.Client {
	if d.client != nil {
		return d.client
	}
	return &http.Client{Timeout: 3 * time.Second}
}
