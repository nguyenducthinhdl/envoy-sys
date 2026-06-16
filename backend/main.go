package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

const testRouteCount = 10000

type response struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

func main() {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	machineName := os.Getenv("MACHINE_NAME")
	if machineName == "" {
		machineName = "machine-1"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	for i := 1; i <= testRouteCount; i++ {
		path := fmt.Sprintf("/test-%d", i)
		message := fmt.Sprintf("test-%d", i)
		mux.HandleFunc(path, handler(machineName, message))
	}

	log.Printf("backend listening on %s with %d /test-{i} routes", addr, testRouteCount)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handler(name, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response{
			Name:    name,
			Message: message,
		})
	}
}
