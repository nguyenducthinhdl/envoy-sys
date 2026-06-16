package snapshot

import (
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

// Config 1: pass-through security filters, no Lua auth.
var config1FilterOrder = []string{
	"ext_authz",
	"jwt_authn",
	"rbac",
	"cors",
	"local_ratelimit",
	"buffer",
	"decompressor",
	"csrf",
	"header_to_metadata",
	"fault",
	"lua_auth",
}

func Config1() ConfigSpec {
	return ConfigSpec{
		Version:        "config-1",
		ClusterName:    "backend_v1",
		ConfigHeader:   "config-1",
		FilterOrder:    config1FilterOrder,
		ConnectTimeout: 5 * time.Second,
		RouteTimeout:   15 * time.Second,
	}
}

func SnapshotConfig1() cache.ResourceSnapshot {
	return BuildSnapshot(Config1())
}
