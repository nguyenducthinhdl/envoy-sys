package envoyviz

import (
	"fmt"
	"strings"
)

const (
	typeClustersConfigDump  = "type.googleapis.com/envoy.admin.v3.ClustersConfigDump"
	typeListenersConfigDump = "type.googleapis.com/envoy.admin.v3.ListenersConfigDump"
	typeEndpointsConfigDump = "type.googleapis.com/envoy.admin.v3.EndpointsConfigDump"
	typeRoutesConfigDump    = "type.googleapis.com/envoy.admin.v3.RoutesConfigDump"
)

// parseRoot parses a decoded config root map into EnvoyConfig.
func parseRoot(root map[string]any) (EnvoyConfig, error) {
	if _, ok := root["configs"]; ok {
		return parseConfigDump(root)
	}
	if _, ok := root["static_resources"]; ok {
		return parseNativeConfig(root)
	}
	if _, ok := root["listeners"]; ok {
		return parseNativeConfig(root)
	}
	if _, ok := root["clusters"]; ok {
		return parseNativeConfig(root)
	}
	return EnvoyConfig{}, fmt.Errorf("unrecognized config shape: expected configs array or static_resources")
}

func parseConfigDump(root map[string]any) (EnvoyConfig, error) {
	configs, ok := asSlice(root["configs"])
	if !ok {
		return EnvoyConfig{}, fmt.Errorf("config dump missing configs array")
	}

	cfg := EnvoyConfig{}
	endpointsByCluster := map[string][]string{}

	for _, item := range configs {
		section, ok := asMap(item)
		if !ok {
			continue
		}
		typ, _ := asString(section["@type"])
		switch typ {
		case typeClustersConfigDump:
			if v, ok := asString(section["version_info"]); ok && cfg.Version == "" {
				cfg.Version = v
			}
			clusters, err := parseClustersSection(section)
			if err != nil {
				return EnvoyConfig{}, err
			}
			cfg.Clusters = append(cfg.Clusters, clusters...)
		case typeListenersConfigDump:
			if v, ok := asString(section["version_info"]); ok && cfg.Version == "" {
				cfg.Version = v
			}
			listeners, err := parseListenersSection(section)
			if err != nil {
				return EnvoyConfig{}, err
			}
			cfg.Listeners = append(cfg.Listeners, listeners...)
		case typeEndpointsConfigDump:
			for name, eps := range parseEndpointsSection(section) {
				endpointsByCluster[name] = append(endpointsByCluster[name], eps...)
			}
		case typeRoutesConfigDump:
			routes, err := parseRoutesSection(section)
			if err != nil {
				return EnvoyConfig{}, err
			}
			cfg.Routes = append(cfg.Routes, routes...)
		}
	}

	for i := range cfg.Clusters {
		if eps, ok := endpointsByCluster[cfg.Clusters[i].Name]; ok {
			cfg.Clusters[i].Endpoints = uniqueStrings(eps)
		}
	}

	return cfg, nil
}

func parseClustersSection(section map[string]any) ([]Cluster, error) {
	var clusters []Cluster

	if static, ok := asSlice(section["static_clusters"]); ok {
		for _, item := range static {
			entry, ok := asMap(item)
			if !ok {
				continue
			}
			clusterMap, ok := asMap(entry["cluster"])
			if !ok {
				continue
			}
			clusters = append(clusters, parseCluster(clusterMap))
		}
	}

	if dynamic, ok := asSlice(section["dynamic_active_clusters"]); ok {
		for _, item := range dynamic {
			entry, ok := asMap(item)
			if !ok {
				continue
			}
			clusterMap, ok := asMap(entry["cluster"])
			if !ok {
				continue
			}
			clusters = append(clusters, parseCluster(clusterMap))
		}
	}

	return clusters, nil
}

func parseListenersSection(section map[string]any) ([]Listener, error) {
	var listeners []Listener

	dynamic, ok := asSlice(section["dynamic_listeners"])
	if !ok {
		return listeners, nil
	}

	for _, item := range dynamic {
		entry, ok := asMap(item)
		if !ok {
			continue
		}
		active, ok := asMap(entry["active_state"])
		if !ok {
			continue
		}
		listenerMap, ok := asMap(active["listener"])
		if !ok {
			continue
		}
		listeners = append(listeners, parseListener(listenerMap))
	}

	return listeners, nil
}

func parseEndpointsSection(section map[string]any) map[string][]string {
	result := map[string][]string{}

	appendEndpoints := func(configs []any) {
		for _, item := range configs {
			entry, ok := asMap(item)
			if !ok {
				continue
			}
			endpointConfig, ok := asMap(entry["endpoint_config"])
			if !ok {
				continue
			}
			name, _ := asString(endpointConfig["cluster_name"])
			if name == "" {
				continue
			}
			result[name] = append(result[name], extractEndpoints(endpointConfig)...)
		}
	}

	if static, ok := asSlice(section["static_endpoint_configs"]); ok {
		appendEndpoints(static)
	}
	if dynamic, ok := asSlice(section["dynamic_endpoint_configs"]); ok {
		appendEndpoints(dynamic)
	}

	return result
}

func parseNativeConfig(root map[string]any) (EnvoyConfig, error) {
	cfg := EnvoyConfig{}

	if static, ok := asMap(root["static_resources"]); ok {
		if listeners, ok := asSlice(static["listeners"]); ok {
			for _, item := range listeners {
				listenerMap, ok := asMap(item)
				if !ok {
					continue
				}
				cfg.Listeners = append(cfg.Listeners, parseListener(listenerMap))
			}
		}
		if clusters, ok := asSlice(static["clusters"]); ok {
			for _, item := range clusters {
				clusterMap, ok := asMap(item)
				if !ok {
					continue
				}
				cfg.Clusters = append(cfg.Clusters, parseCluster(clusterMap))
			}
		}
		if routes, ok := asSlice(static["routes"]); ok {
			for _, item := range routes {
				routeMap, ok := asMap(item)
				if !ok {
					continue
				}
				cfg.Routes = append(cfg.Routes, parseRouteConfig(routeMap))
			}
		}
	}

	if listeners, ok := asSlice(root["listeners"]); ok {
		for _, item := range listeners {
			listenerMap, ok := asMap(item)
			if !ok {
				continue
			}
			cfg.Listeners = append(cfg.Listeners, parseListener(listenerMap))
		}
	}

	if clusters, ok := asSlice(root["clusters"]); ok {
		for _, item := range clusters {
			clusterMap, ok := asMap(item)
			if !ok {
				continue
			}
			cfg.Clusters = append(cfg.Clusters, parseCluster(clusterMap))
		}
	}

	return cfg, nil
}

func parseListener(listenerMap map[string]any) Listener {
	name, _ := asString(listenerMap["name"])
	address := formatSocketAddress(listenerMap["address"])

	var chains []FilterChain
	if filterChains, ok := asSlice(listenerMap["filter_chains"]); ok {
		for _, item := range filterChains {
			chainMap, ok := asMap(item)
			if !ok {
				continue
			}
			chains = append(chains, parseFilterChain(chainMap))
		}
	}

	return Listener{
		Name:         name,
		Address:      address,
		FilterChains: chains,
	}
}

func parseFilterChain(chainMap map[string]any) FilterChain {
	chain := FilterChain{}

	if filters, ok := asSlice(chainMap["filters"]); ok {
		for _, item := range filters {
			filterMap, ok := asMap(item)
			if !ok {
				continue
			}
			name, _ := asString(filterMap["name"])
			chain.NetworkFilters = append(chain.NetworkFilters, name)

			if strings.Contains(name, "http_connection_manager") {
				if typed, ok := asMap(filterMap["typed_config"]); ok {
					chain.HttpFilters = parseHTTPFilters(typed)
					chain.RouteConfigName = extractRouteConfigName(typed)
				}
			}
		}
	}

	return chain
}

func parseHTTPFilters(hcm map[string]any) []HttpFilter {
	var filters []HttpFilter
	httpFilters, ok := asSlice(hcm["http_filters"])
	if !ok {
		return filters
	}

	for _, item := range httpFilters {
		filterMap, ok := asMap(item)
		if !ok {
			continue
		}
		fullName, _ := asString(filterMap["name"])
		filters = append(filters, HttpFilter{
			Name:     shortFilterName(fullName),
			FullName: fullName,
		})
	}

	return filters
}

func parseRoutesSection(section map[string]any) ([]RouteConfig, error) {
	var routes []RouteConfig

	appendRoutes := func(configs []any) {
		for _, item := range configs {
			entry, ok := asMap(item)
			if !ok {
				continue
			}
			routeMap, ok := asMap(entry["route_config"])
			if !ok {
				continue
			}
			routes = append(routes, parseRouteConfig(routeMap))
		}
	}

	if static, ok := asSlice(section["static_route_configs"]); ok {
		appendRoutes(static)
	}
	if dynamic, ok := asSlice(section["dynamic_route_configs"]); ok {
		appendRoutes(dynamic)
	}

	return routes, nil
}

func parseRouteConfig(routeMap map[string]any) RouteConfig {
	name, _ := asString(routeMap["name"])
	cfg := RouteConfig{Name: name}

	virtualHosts, ok := asSlice(routeMap["virtual_hosts"])
	if !ok {
		return cfg
	}

	for _, item := range virtualHosts {
		vhostMap, ok := asMap(item)
		if !ok {
			continue
		}
		cfg.VirtualHosts = append(cfg.VirtualHosts, parseVirtualHost(vhostMap))
	}

	return cfg
}

func parseVirtualHost(vhostMap map[string]any) VirtualHost {
	name, _ := asString(vhostMap["name"])
	vhost := VirtualHost{Name: name}

	if domains, ok := asSlice(vhostMap["domains"]); ok {
		for _, item := range domains {
			if domain, ok := asString(item); ok {
				vhost.Domains = append(vhost.Domains, domain)
			}
		}
	}

	if routes, ok := asSlice(vhostMap["routes"]); ok {
		for _, item := range routes {
			routeMap, ok := asMap(item)
			if !ok {
				continue
			}
			vhost.Routes = append(vhost.Routes, parseRoute(routeMap))
		}
	}

	return vhost
}

func parseRoute(routeMap map[string]any) Route {
	route := Route{}

	if match, ok := asMap(routeMap["match"]); ok {
		route.Match = formatRouteMatch(match)
	}

	if action, ok := asMap(routeMap["route"]); ok {
		route.Cluster, _ = asString(action["cluster"])
		if route.Cluster == "" {
			if clusterSpecifier, ok := asMap(action["weighted_clusters"]); ok {
				route.Cluster = formatWeightedClusters(clusterSpecifier)
			}
		}
		route.Timeout, _ = asString(action["timeout"])
	}

	if route.Cluster == "" {
		if cluster, ok := asString(routeMap["cluster"]); ok {
			route.Cluster = cluster
		}
	}

	return route
}

func formatRouteMatch(match map[string]any) string {
	if prefix, ok := asString(match["prefix"]); ok {
		return "prefix:" + prefix
	}
	if path, ok := asString(match["path"]); ok {
		return "path:" + path
	}
	if regex, ok := asString(match["safe_regex"]); ok {
		return "regex:" + regex
	}
	if regexMap, ok := asMap(match["safe_regex"]); ok {
		if regex, ok := asString(regexMap["regex"]); ok {
			return "regex:" + regex
		}
	}
	if connect, ok := asMap(match["connect_matcher"]); ok {
		if prefix, ok := asString(connect["prefix"]); ok {
			return "connect:" + prefix
		}
	}
	return "match"
}

func formatWeightedClusters(spec map[string]any) string {
	clusters, ok := asSlice(spec["clusters"])
	if !ok {
		return "weighted"
	}
	var names []string
	for _, item := range clusters {
		clusterMap, ok := asMap(item)
		if !ok {
			continue
		}
		if name, ok := asString(clusterMap["name"]); ok {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "weighted"
	}
	return "weighted(" + strings.Join(names, ",") + ")"
}

func extractRouteConfigName(hcm map[string]any) string {
	if rds, ok := asMap(hcm["rds"]); ok {
		if name, ok := asString(rds["route_config_name"]); ok {
			return name
		}
	}
	if routeConfig, ok := asMap(hcm["route_config"]); ok {
		if name, ok := asString(routeConfig["name"]); ok {
			return name
		}
	}
	return ""
}

func parseCluster(clusterMap map[string]any) Cluster {
	name, _ := asString(clusterMap["name"])
	clusterType, _ := asString(clusterMap["type"])
	connectTimeout, _ := asString(clusterMap["connect_timeout"])

	cluster := Cluster{
		Name:           name,
		Type:           clusterType,
		ConnectTimeout: connectTimeout,
	}

	if loadAssignment, ok := asMap(clusterMap["load_assignment"]); ok {
		cluster.Endpoints = extractEndpoints(loadAssignment)
	}

	return cluster
}

func extractEndpoints(container map[string]any) []string {
	var endpoints []string
	endpointItems, ok := asSlice(container["endpoints"])
	if !ok {
		return endpoints
	}

	for _, item := range endpointItems {
		endpointGroup, ok := asMap(item)
		if !ok {
			continue
		}
		lbEndpoints, ok := asSlice(endpointGroup["lb_endpoints"])
		if !ok {
			continue
		}
		for _, lbItem := range lbEndpoints {
			lbEndpoint, ok := asMap(lbItem)
			if !ok {
				continue
			}
			endpoint, ok := asMap(lbEndpoint["endpoint"])
			if !ok {
				continue
			}
			addr := formatSocketAddress(endpoint["address"])
			if addr != "" {
				endpoints = append(endpoints, addr)
			}
		}
	}

	return uniqueStrings(endpoints)
}

func formatSocketAddress(value any) string {
	addrMap, ok := asMap(value)
	if !ok {
		return ""
	}

	if socket, ok := asMap(addrMap["socket_address"]); ok {
		host, _ := asString(socket["address"])
		port := formatPort(socket["port_value"])
		if host == "" {
			return ""
		}
		if port == "" {
			return host
		}
		return host + ":" + port
	}

	return ""
}

func formatPort(value any) string {
	switch v := value.(type) {
	case float64:
		return fmt.Sprintf("%d", int(v))
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case string:
		return v
	default:
		return ""
	}
}

func shortFilterName(fullName string) string {
	const prefix = "envoy.filters.http."
	if strings.HasPrefix(fullName, prefix) {
		return strings.TrimPrefix(fullName, prefix)
	}
	const networkPrefix = "envoy.filters.network."
	if strings.HasPrefix(fullName, networkPrefix) {
		return strings.TrimPrefix(fullName, networkPrefix)
	}
	return fullName
}

func asMap(value any) (map[string]any, bool) {
	m, ok := value.(map[string]any)
	return m, ok
}

func asSlice(value any) ([]any, bool) {
	s, ok := value.([]any)
	return s, ok
}

func asString(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case fmt.Stringer:
		return v.String(), true
	default:
		return "", false
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var result []string
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
