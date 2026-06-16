package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	envoyviz "github.com/demo/envoyviz"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

func envoyAdminURLs() []string {
	var urls []string
	add := func(u string) {
		u = strings.TrimSuffix(strings.TrimSpace(u), "/")
		if u == "" {
			return
		}
		for _, existing := range urls {
			if existing == u {
				return
			}
		}
		urls = append(urls, u)
	}

	add(os.Getenv("ENVOY_ADMIN_URL"))
	if len(urls) == 0 {
		add("http://app-sidecar:9901")
	}
	add(os.Getenv("ENVOY_ADMIN_FALLBACK_URL"))
	if os.Getenv("ENVOY_ADMIN_FALLBACK_URL") == "" {
		add("http://host.docker.internal:9901")
	}

	return urls
}

func fetchEnvoyConfigDump() ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	path := "/config_dump?include_eds=1"

	var lastErr error
	for _, base := range envoyAdminURLs() {
		url := base + path
		for attempt := 1; attempt <= 3; attempt++ {
			resp, err := client.Get(url)
			if err != nil {
				lastErr = fmt.Errorf("%s: %w", base, err)
				time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
				continue
			}

			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
				continue
			}
			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("%s returned %d: %s", base, resp.StatusCode, string(body))
				continue
			}
			return body, nil
		}
	}

	return nil, fmt.Errorf(
		"envoy admin unreachable — ensure app-sidecar is running (docker-compose up -d app-sidecar): %v",
		lastErr,
	)
}

func formatEnvoyTree(dump []byte) (string, error) {
	cfg, err := envoyviz.Parse(dump, "config-dump.json")
	if err != nil {
		return "", fmt.Errorf("parse config dump: %w", err)
	}
	return envoyviz.FormatConfig(cfg), nil
}

func handleEnvoyConfigTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeConfigTree(w, false, nil, nil, "")
}

func handleConfigTree(
	w http.ResponseWriter,
	r *http.Request,
	snapshotCache cache.SnapshotCache,
	build func() cache.ResourceSnapshot,
	label string,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeConfigTree(w, true, snapshotCache, build, label)
}

func writeConfigTree(
	w http.ResponseWriter,
	push bool,
	snapshotCache cache.SnapshotCache,
	build func() cache.ResourceSnapshot,
	label string,
) {
	if push {
		if err := pushSnapshot(snapshotCache, build()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		time.Sleep(750 * time.Millisecond)
	}

	dump, err := fetchEnvoyConfigDump()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	tree, err := formatEnvoyTree(dump)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if label != "" {
		_, _ = w.Write([]byte("# " + label + " (live sidecar)\n\n"))
	}
	_, _ = w.Write([]byte(tree))
}
