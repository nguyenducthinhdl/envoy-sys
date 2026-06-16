package snapshot

import (
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

// Config 2: different filter order + Lua auth that always returns 401.
var config2FilterOrder = []string{
	"fault",
	"header_to_metadata",
	"csrf",
	"decompressor",
	"buffer",
	"local_ratelimit",
	"cors",
	"rbac",
	"jwt_authn",
	"ext_authz",
	"lua_auth",
}

func Config2() ConfigSpec {
	return ConfigSpec{
		Version:        "config-2",
		ClusterName:    "backend_v2",
		ConfigHeader:   "config-2",
		FilterOrder:    config2FilterOrder,
		ConnectTimeout: 2 * time.Second,
		RouteTimeout:   5 * time.Second,
	}
}

func SnapshotConfig2() cache.ResourceSnapshot {
	return BuildSnapshot(Config2())
}
