package envoyviz

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigDumpJSON(t *testing.T) {
	cfg := mustParseFile(t, "testdata/config-dump-1.json")

	if cfg.Version != "config-1" {
		t.Fatalf("version = %q, want config-1", cfg.Version)
	}
	if len(cfg.Listeners) == 0 {
		t.Fatal("expected listeners")
	}
	if cfg.Listeners[0].Name != "listener_8081" {
		t.Fatalf("listener name = %q", cfg.Listeners[0].Name)
	}
	if cfg.Listeners[0].Address != "0.0.0.0:8081" {
		t.Fatalf("listener address = %q", cfg.Listeners[0].Address)
	}

	names := filterNames(cfg.Listeners[0].FilterChains[0].HttpFilters)
	want := []string{
		"ext_authz", "jwt_authn", "rbac", "cors", "local_ratelimit",
		"buffer", "decompressor", "csrf", "header_to_metadata", "fault", "router",
	}
	if len(names) != len(want) {
		t.Fatalf("http filters = %v, want %v", names, want)
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("http filter[%d] = %q, want %q", i, names[i], name)
		}
	}

	cluster := findCluster(cfg.Clusters, "backend_v1")
	if cluster == nil {
		t.Fatal("backend_v1 cluster not found")
	}
	if cluster.ConnectTimeout != "5s" {
		t.Fatalf("connect_timeout = %q, want 5s", cluster.ConnectTimeout)
	}

	if cfg.Listeners[0].FilterChains[0].RouteConfigName != "local_route" {
		t.Fatalf("route_config_name = %q, want local_route", cfg.Listeners[0].FilterChains[0].RouteConfigName)
	}

	route := findRoute(cfg.Routes, "local_route")
	if route == nil {
		t.Fatal("local_route not found")
	}
	if len(route.VirtualHosts) == 0 || len(route.VirtualHosts[0].Routes) < 2 {
		t.Fatalf("expected virtual host routes, got %+v", route.VirtualHosts)
	}
	if route.VirtualHosts[0].Routes[0].Match != "prefix:/test-1" {
		t.Fatalf("route match = %q", route.VirtualHosts[0].Routes[0].Match)
	}
	if route.VirtualHosts[0].Routes[0].Cluster != "backend_v1" {
		t.Fatalf("route cluster = %q", route.VirtualHosts[0].Routes[0].Cluster)
	}
}

func TestParseConfigDumpYAML(t *testing.T) {
	if _, err := os.Stat("testdata/config-dump-1.yaml"); os.IsNotExist(err) {
		t.Skip("testdata/config-dump-1.yaml not generated")
	}

	jsonCfg := mustParseFile(t, "testdata/config-dump-1.json")
	yamlCfg := mustParseFile(t, "testdata/config-dump-1.yaml")

	if jsonCfg.Listeners[0].Name != yamlCfg.Listeners[0].Name {
		t.Fatalf("yaml listener mismatch: %q vs %q", yamlCfg.Listeners[0].Name, jsonCfg.Listeners[0].Name)
	}
	if len(jsonCfg.Listeners[0].FilterChains[0].HttpFilters) != len(yamlCfg.Listeners[0].FilterChains[0].HttpFilters) {
		t.Fatal("yaml http filter count mismatch")
	}
}

func TestParseBootstrapYAML(t *testing.T) {
	cfg := mustParseFile(t, "testdata/bootstrap.yaml")

	if len(cfg.Listeners) != 0 {
		t.Fatalf("expected no listeners, got %d", len(cfg.Listeners))
	}
	cluster := findCluster(cfg.Clusters, "xds_cluster")
	if cluster == nil {
		t.Fatal("xds_cluster not found")
	}
	if cluster.ConnectTimeout != "5s" {
		t.Fatalf("connect_timeout = %q, want 5s", cluster.ConnectTimeout)
	}
}

func TestParseFolderMerge(t *testing.T) {
	fileCfg := mustParseFile(t, "testdata/config-dump-1.json")
	folderCfg := mustParsePath(t, "testdata/config-1")

	if folderCfg.Listeners[0].Name != fileCfg.Listeners[0].Name {
		t.Fatalf("folder listener = %q, want %q", folderCfg.Listeners[0].Name, fileCfg.Listeners[0].Name)
	}
	if findCluster(folderCfg.Clusters, "backend_v1") == nil {
		t.Fatal("folder merge missing backend_v1")
	}
	if findCluster(folderCfg.Clusters, "xds_cluster") == nil {
		t.Fatal("folder merge missing xds_cluster from bootstrap")
	}
}

func mustParseFile(t *testing.T, path string) EnvoyConfig {
	t.Helper()
	cfg, err := ParsePath(path, ParseOptions{})
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return cfg
}

func mustParsePath(t *testing.T, path string) EnvoyConfig {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return mustParseFile(t, abs)
}

func findRoute(routes []RouteConfig, name string) *RouteConfig {
	for i := range routes {
		if routes[i].Name == name {
			return &routes[i]
		}
	}
	return nil
}

func findCluster(clusters []Cluster, name string) *Cluster {
	for i := range clusters {
		if clusters[i].Name == name {
			return &clusters[i]
		}
	}
	return nil
}
