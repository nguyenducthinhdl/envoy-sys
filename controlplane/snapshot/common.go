package snapshot

import (
	"fmt"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	rbacconfig "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	gzipdecomp "github.com/envoyproxy/go-control-plane/envoy/extensions/compression/gzip/decompressor/v3"
	buffer "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/buffer/v3"
	cors "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/cors/v3"
	csrf "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/csrf/v3"
	httpdecomp "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/decompressor/v3"
	extauthz "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	fault "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/fault/v3"
	headermeta "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/header_to_metadata/v3"
	jwtauthn "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	localratelimit "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/local_ratelimit/v3"
	lua "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/lua/v3"
	rbac "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/rbac/v3"
	router "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	NodeID       = "sidecar-1"
	ListenerName = "listener_8081"
	RouteName    = "local_route"
	UpstreamHost = "127.0.0.1"
	UpstreamPort = 8080
	ListenerPort = 8081
	AuthzCluster = "ext_authz_cluster"
)

type ConfigSpec struct {
	Version        string
	ClusterName    string
	ConfigHeader   string
	FilterOrder    []string
	ConnectTimeout time.Duration
	RouteTimeout   time.Duration
}

func BuildSnapshot(spec ConfigSpec) cache.ResourceSnapshot {
	snap, err := cache.NewSnapshot(
		spec.Version,
		map[resource.Type][]types.Resource{
			resource.EndpointType: {makeEDS(spec.ClusterName)},
			resource.ClusterType:  {makeCluster(spec), makeAuthzCluster()},
			resource.RouteType:    {makeRouteWithCluster(spec)},
			resource.ListenerType: {makeListener(spec)},
		},
	)
	if err != nil {
		panic(err)
	}
	return snap
}

func makeAuthzCluster() *cluster.Cluster {
	return &cluster.Cluster{
		Name:           AuthzCluster,
		ConnectTimeout: durationpb.New(1 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{
			Type: cluster.Cluster_STATIC,
		},
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: AuthzCluster,
			Endpoints: []*endpoint.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpoint.LbEndpoint{
						{
							HostIdentifier: &endpoint.LbEndpoint_Endpoint{
								Endpoint: &endpoint.Endpoint{
									Address: socketAddress("127.0.0.1", 9999),
								},
							},
						},
					},
				},
			},
		},
	}
}

func makeCluster(spec ConfigSpec) *cluster.Cluster {
	timeout := spec.ConnectTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &cluster.Cluster{
		Name:           spec.ClusterName,
		ConnectTimeout: durationpb.New(timeout),
		ClusterDiscoveryType: &cluster.Cluster_Type{
			Type: cluster.Cluster_EDS,
		},
		EdsClusterConfig: &cluster.Cluster_EdsClusterConfig{
			EdsConfig:   adsConfig(),
			ServiceName: spec.ClusterName,
		},
		LbPolicy: cluster.Cluster_ROUND_ROBIN,
	}
}

func makeEDS(clusterName string) *endpoint.ClusterLoadAssignment {
	return &endpoint.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpoint.LocalityLbEndpoints{
			{
				LbEndpoints: []*endpoint.LbEndpoint{
					{
						HostIdentifier: &endpoint.LbEndpoint_Endpoint{
							Endpoint: &endpoint.Endpoint{
								Address: socketAddress(UpstreamHost, UpstreamPort),
							},
						},
					},
				},
			},
		},
	}
}

func makeRouteWithCluster(spec ConfigSpec) *route.RouteConfiguration {
	timeout := spec.RouteTimeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	routes := make([]*route.Route, 0, TestRouteCount)
	for i := 1; i <= TestRouteCount; i++ {
		path := fmt.Sprintf("/test-%d", i)
		routes = append(routes, routeForPath(path, spec.ClusterName, spec.ConfigHeader, timeout))
	}
	return &route.RouteConfiguration{
		Name: RouteName,
		VirtualHosts: []*route.VirtualHost{
			{
				Name:    "local_service",
				Domains: []string{"*"},
				Routes:  routes,
			},
		},
	}
}

func routeForPath(prefix, clusterName, configHeader string, timeout time.Duration) *route.Route {
	return &route.Route{
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{Prefix: prefix},
		},
		Action: &route.Route_Route{
			Route: &route.RouteAction{
				ClusterSpecifier: &route.RouteAction_Cluster{Cluster: clusterName},
				Timeout:          durationpb.New(timeout),
			},
		},
		ResponseHeadersToAdd: []*core.HeaderValueOption{
			{Header: &core.HeaderValue{Key: "x-demo-config", Value: configHeader}},
		},
	}
}

func makeListener(spec ConfigSpec) *listener.Listener {
	chains := make([]*listener.FilterChain, 0, FilterChainCount+1)
	for i := 1; i <= FilterChainCount; i++ {
		chains = append(chains, &listener.FilterChain{
			FilterChainMatch: &listener.FilterChainMatch{
				SourcePrefixRanges: []*core.CidrRange{sourcePrefixForChain(i)},
			},
			Filters: []*listener.Filter{
				hcmNetworkFilter(spec, fmt.Sprintf("ingress_http_fc_%d", i)),
			},
		})
	}
	// Catch-all chain for real client traffic (no source-prefix match).
	chains = append(chains, &listener.FilterChain{
		Filters: []*listener.Filter{
			hcmNetworkFilter(spec, "ingress_http"),
		},
	})

	return &listener.Listener{
		Name: ListenerName,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: ListenerPort,
					},
				},
			},
		},
		FilterChains: chains,
	}
}

func sourcePrefixForChain(i int) *core.CidrRange {
	// Synthetic /32 ranges that never match demo traffic; only used to pad LDS.
	octet2 := (i - 1) / 254
	octet3 := ((i - 1) % 254) + 1
	return &core.CidrRange{
		AddressPrefix: fmt.Sprintf("10.%d.%d.1", octet2, octet3),
		PrefixLen:     wrapperspb.UInt32(32),
	}
}

func hcmNetworkFilter(spec ConfigSpec, statPrefix string) *listener.Filter {
	mgr, _ := anypb.New(&hcm.HttpConnectionManager{
		StatPrefix: statPrefix,
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource:    adsConfigSource(),
				RouteConfigName: RouteName,
			},
		},
		HttpFilters: buildHTTPFilters(spec.FilterOrder),
	})
	return &listener.Filter{
		Name: "envoy.filters.network.http_connection_manager",
		ConfigType: &listener.Filter_TypedConfig{
			TypedConfig: mgr,
		},
	}
}

func buildHTTPFilters(order []string) []*hcm.HttpFilter {
	filters := make([]*hcm.HttpFilter, 0, len(order)+1)
	for _, name := range order {
		f, err := filterByName(name)
		if err != nil {
			panic(err)
		}
		filters = append(filters, f)
	}
	routerFilter, _ := anypb.New(&router.Router{})
	filters = append(filters, &hcm.HttpFilter{
		Name: "envoy.filters.http.router",
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: routerFilter,
		},
	})
	return filters
}

func filterByName(name string) (*hcm.HttpFilter, error) {
	switch name {
	case "lua_auth":
		return wrapFilter("envoy.filters.http.lua", &lua.Lua{
			StatPrefix: "lua_auth",
			DefaultSourceCode: &core.DataSource{
				Specifier: &core.DataSource_InlineString{
					InlineString: luaAuthScript,
				},
			},
		})
	case "ext_authz":
		return wrapFilter("envoy.filters.http.ext_authz", &extauthz.ExtAuthz{
			TransportApiVersion: core.ApiVersion_V3,
			FailureModeAllow:    true,
			WithRequestBody:     &extauthz.BufferSettings{MaxRequestBytes: 8192},
			Services: &extauthz.ExtAuthz_GrpcService{
				GrpcService: &core.GrpcService{
					TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
						EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
							ClusterName: AuthzCluster,
						},
					},
					Timeout: durationpb.New(250 * time.Millisecond),
				},
			},
		})
	case "jwt_authn":
		return wrapFilter("envoy.filters.http.jwt_authn", &jwtauthn.JwtAuthentication{})
	case "rbac":
		return wrapFilter("envoy.filters.http.rbac", &rbac.RBAC{
			Rules: &rbacconfig.RBAC{
				Action: rbacconfig.RBAC_ALLOW,
				Policies: map[string]*rbacconfig.Policy{
					"allow_all": {
						Permissions: []*rbacconfig.Permission{
							{Rule: &rbacconfig.Permission_Any{Any: true}},
						},
						Principals: []*rbacconfig.Principal{
							{Identifier: &rbacconfig.Principal_Any{Any: true}},
						},
					},
				},
			},
		})
	case "cors":
		return wrapFilter("envoy.filters.http.cors", &cors.Cors{})
	case "local_ratelimit":
		return wrapFilter("envoy.filters.http.local_ratelimit", &localratelimit.LocalRateLimit{
			StatPrefix: "local_rate_limiter",
			TokenBucket: &typev3.TokenBucket{
				MaxTokens:     10000,
				TokensPerFill: &wrapperspb.UInt32Value{Value: 10000},
				FillInterval:  durationpb.New(time.Second),
			},
			FilterEnabled: &core.RuntimeFractionalPercent{
				DefaultValue: &typev3.FractionalPercent{
					Numerator:   100,
					Denominator: typev3.FractionalPercent_HUNDRED,
				},
			},
			FilterEnforced: &core.RuntimeFractionalPercent{
				DefaultValue: &typev3.FractionalPercent{
					Numerator:   100,
					Denominator: typev3.FractionalPercent_HUNDRED,
				},
			},
		})
	case "buffer":
		return wrapFilter("envoy.filters.http.buffer", &buffer.Buffer{
			MaxRequestBytes: &wrapperspb.UInt32Value{Value: 1048576},
		})
	case "decompressor":
		gzipLib, _ := anypb.New(&gzipdecomp.Gzip{})
		return wrapFilter("envoy.filters.http.decompressor", &httpdecomp.Decompressor{
			DecompressorLibrary: &core.TypedExtensionConfig{
				Name:        "envoy.compression.gzip.decompressor",
				TypedConfig: gzipLib,
			},
		})
	case "csrf":
		return wrapFilter("envoy.filters.http.csrf", &csrf.CsrfPolicy{
			FilterEnabled: &core.RuntimeFractionalPercent{
				DefaultValue: &typev3.FractionalPercent{
					Numerator:   0,
					Denominator: typev3.FractionalPercent_HUNDRED,
				},
			},
		})
	case "header_to_metadata":
		return wrapFilter("envoy.filters.http.header_to_metadata", &headermeta.Config{
			RequestRules: []*headermeta.Config_Rule{
				{
					Header: "x-request-id",
					OnHeaderPresent: &headermeta.Config_KeyValuePair{
						MetadataNamespace: "envoy.security",
						Key:               "request_id",
						Type:              headermeta.Config_STRING,
					},
				},
			},
		})
	case "fault":
		return wrapFilter("envoy.filters.http.fault", &fault.HTTPFault{
			MaxActiveFaults: &wrapperspb.UInt32Value{Value: 0},
			Abort: &fault.FaultAbort{
				Percentage: &typev3.FractionalPercent{
					Numerator:   0,
					Denominator: typev3.FractionalPercent_HUNDRED,
				},
				ErrorType: &fault.FaultAbort_HttpStatus{
					HttpStatus: 503,
				},
			},
		})
	default:
		return nil, fmt.Errorf("unknown filter: %s", name)
	}
}

func wrapFilter(name string, msg proto.Message) (*hcm.HttpFilter, error) {
	cfg, err := anypb.New(msg)
	if err != nil {
		return nil, err
	}
	return &hcm.HttpFilter{
		Name: name,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: cfg,
		},
	}, nil
}

func adsConfig() *core.ConfigSource {
	return &core.ConfigSource{
		ResourceApiVersion: resource.DefaultAPIVersion,
		ConfigSourceSpecifier: &core.ConfigSource_Ads{
			Ads: &core.AggregatedConfigSource{},
		},
	}
}

func adsConfigSource() *core.ConfigSource {
	return adsConfig()
}

func socketAddress(host string, port uint32) *core.Address {
	return &core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: core.SocketAddress_TCP,
				Address:  host,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: port,
				},
			},
		},
	}
}
