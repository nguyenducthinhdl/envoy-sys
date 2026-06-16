package envoyviz

// EnvoyConfig is a normalized view of Envoy listeners, routes, and clusters for visualization.
type EnvoyConfig struct {
	Version   string
	Listeners []Listener
	Routes    []RouteConfig
	Clusters  []Cluster
}

// RouteConfig is a named route configuration (RDS).
type RouteConfig struct {
	Name          string
	VirtualHosts  []VirtualHost
}

// VirtualHost groups routes for a set of domains.
type VirtualHost struct {
	Name    string
	Domains []string
	Routes  []Route
}

// Route maps a request match to a destination cluster.
type Route struct {
	Match   string
	Cluster string
	Timeout string
}

// Listener represents an Envoy listener with filter chains.
type Listener struct {
	Name         string
	Address      string
	FilterChains []FilterChain
}

// FilterChain holds network and HTTP filters for a listener filter chain.
type FilterChain struct {
	RouteConfigName string
	NetworkFilters  []string
	HttpFilters     []HttpFilter
}

// HttpFilter is an HTTP filter in the HCM filter chain.
type HttpFilter struct {
	Name     string
	FullName string
}

// Cluster represents an upstream cluster.
type Cluster struct {
	Name            string
	Type            string
	ConnectTimeout  string
	Endpoints       []string
}
