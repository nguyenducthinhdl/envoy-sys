package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	"github.com/demo/envoy-xds-demo/controlplane/snapshot"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, nil)
	cb := &test.Callbacks{Debug: false}
	srv := server.NewServer(ctx, snapshotCache, cb)

	grpcServer := grpc.NewServer()
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, srv)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, srv)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, srv)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, srv)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, srv)

	lis, err := net.Listen("tcp", net.JoinHostPort("", grpcPortString()))
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}

	go func() {
		log.Printf("xDS server listening on :%s", grpcPortString())
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
	}()

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	adminMux.HandleFunc("/config/1", func(w http.ResponseWriter, r *http.Request) {
		handlePushConfig(w, r, snapshotCache, snapshot.SnapshotConfig1, "config-1")
	})
	adminMux.HandleFunc("/config/2", func(w http.ResponseWriter, r *http.Request) {
		handlePushConfig(w, r, snapshotCache, snapshot.SnapshotConfig2, "config-2")
	})

	adminServer := &http.Server{
		Addr:              net.JoinHostPort("", httpPortString()),
		Handler:           adminMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("HTTP admin listening on :%s", httpPortString())
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http serve: %v", err)
		}
	}()

	guiServer := &http.Server{
		Addr:              net.JoinHostPort("", guiPortString()),
		Handler:           newGUIMux(snapshotCache),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("GUI listening on :%s", guiPortString())
		if err := guiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gui serve: %v", err)
		}
	}()

	if err := pushSnapshot(snapshotCache, snapshot.SnapshotConfig1()); err != nil {
		log.Fatalf("initial snapshot: %v", err)
	}
	log.Printf("preloaded config-1 for node %s", snapshot.NodeID)

	<-ctx.Done()
	_ = adminServer.Shutdown(context.Background())
	_ = guiServer.Shutdown(context.Background())
	grpcServer.GracefulStop()
}

func handlePushConfig(
	w http.ResponseWriter,
	r *http.Request,
	c cache.SnapshotCache,
	build func() cache.ResourceSnapshot,
	name string,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := pushSnapshot(c, build()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("pushed %s to node %s", name, snapshot.NodeID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","config":"` + name + `"}`))
}

func pushSnapshot(c cache.SnapshotCache, snap cache.ResourceSnapshot) error {
	return c.SetSnapshot(context.Background(), snapshot.NodeID, snap)
}

func grpcPortString() string {
	if v := os.Getenv("GRPC_PORT"); v != "" {
		return v
	}
	return "18000"
}

func httpPortString() string {
	if v := os.Getenv("HTTP_PORT"); v != "" {
		return v
	}
	return "8080"
}
