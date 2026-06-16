package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"

	"github.com/demo/envoy-xds-demo/controlplane/snapshot"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

//go:embed gui/*
var guiFS embed.FS

func guiPortString() string {
	if v := os.Getenv("GUI_PORT"); v != "" {
		return v
	}
	return "8022"
}

func envoyAdminURL() string {
	urls := envoyAdminURLs()
	if len(urls) == 0 {
		return "http://app-sidecar:9901"
	}
	return urls[0]
}

func newGUIMux(snapshotCache cache.SnapshotCache) http.Handler {
	mux := http.NewServeMux()

	sub, _ := fs.Sub(guiFS, "gui")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	mux.HandleFunc("/api/config/1", func(w http.ResponseWriter, r *http.Request) {
		handlePushConfig(w, r, snapshotCache, snapshot.SnapshotConfig1, "config-1")
	})
	mux.HandleFunc("/api/config/2", func(w http.ResponseWriter, r *http.Request) {
		handlePushConfig(w, r, snapshotCache, snapshot.SnapshotConfig2, "config-2")
	})
	mux.HandleFunc("/api/config/1/tree", func(w http.ResponseWriter, r *http.Request) {
		handleConfigTree(w, r, snapshotCache, snapshot.SnapshotConfig1, "config-1")
	})
	mux.HandleFunc("/api/config/2/tree", func(w http.ResponseWriter, r *http.Request) {
		handleConfigTree(w, r, snapshotCache, snapshot.SnapshotConfig2, "config-2")
	})
	mux.HandleFunc("/api/envoy/config-dump", handleEnvoyConfigDump)
	mux.HandleFunc("/api/envoy/config-tree", handleEnvoyConfigTree)

	return mux
}

func handleEnvoyConfigDump(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := fetchEnvoyConfigDump()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}
