package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/sdomino/scribble"
)

type note struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	data := flag.String("data", "./data", "database directory")
	id := flag.String("id", "node-1", "node identifier")
	peers := flag.String("peers", "", "comma-separated peer base URLs")
	syncFrom := flag.String("sync-from", "", "peer URL used for bootstrap sync")
	flag.Parse()

	db, err := scribble.New(*data, &scribble.Options{
		NodeID: *id,
		Peers:  splitPeers(*peers),
	})
	if err != nil {
		log.Fatal(err)
	}
	if *syncFrom != "" {
		if err := db.SyncFrom(*syncFrom); err != nil {
			log.Fatal(err)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/scribble/", db.HTTPHandler())
	mux.HandleFunc("/notes/", func(w http.ResponseWriter, r *http.Request) {
		resource := strings.TrimPrefix(r.URL.Path, "/notes/")
		switch r.Method {
		case http.MethodPut:
			var value note
			if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := db.Write("notes", resource, value); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			var value note
			if err := db.Read("notes", resource, &value); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(value)
		case http.MethodDelete:
			if err := db.Delete("notes", resource); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	fmt.Printf("%s listening on %s, peers=%v\n", *id, *addr, splitPeers(*peers))
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func splitPeers(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, strings.TrimSpace(item))
	}
	return result
}
