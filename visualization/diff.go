package envoyviz

import (
	"fmt"
	"sort"
	"strings"
)

// DiffKind describes a diff entry type.
type DiffKind string

const (
	DiffAdded   DiffKind = "added"
	DiffRemoved DiffKind = "removed"
	DiffChanged DiffKind = "changed"
)

// DiffEntry is a single difference between two configs.
type DiffEntry struct {
	Path     string
	Kind     DiffKind
	OldValue string
	NewValue string
}

// DiffResult holds all differences between two configs.
type DiffResult struct {
	Entries []DiffEntry
}

// HasDiffs reports whether any differences were found.
func (r DiffResult) HasDiffs() bool {
	return len(r.Entries) > 0
}

// Compare returns structural differences between left and right configs.
func Compare(left, right EnvoyConfig) DiffResult {
	var entries []DiffEntry

	entries = append(entries, diffListeners(left.Listeners, right.Listeners)...)
	entries = append(entries, diffRoutes(left.Routes, right.Routes)...)
	entries = append(entries, diffClusters(left.Clusters, right.Clusters)...)

	if left.Version != right.Version && (left.Version != "" || right.Version != "") {
		entries = append(entries, DiffEntry{
			Path:     "Version",
			Kind:     DiffChanged,
			OldValue: left.Version,
			NewValue: right.Version,
		})
	}

	return DiffResult{Entries: entries}
}

func diffListeners(left, right []Listener) []DiffEntry {
	leftByName := indexListeners(left)
	rightByName := indexListeners(right)

	var names []string
	seen := map[string]struct{}{}
	for name := range leftByName {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for name := range rightByName {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var entries []DiffEntry
	for _, name := range names {
		l, lok := leftByName[name]
		r, rok := rightByName[name]
		base := "Listeners/" + name

		switch {
		case lok && !rok:
			entries = append(entries, DiffEntry{Path: base, Kind: DiffRemoved, OldValue: l.Address})
		case !lok && rok:
			entries = append(entries, DiffEntry{Path: base, Kind: DiffAdded, NewValue: r.Address})
		default:
			if l.Address != r.Address {
				entries = append(entries, DiffEntry{
					Path:     base + "/Address",
					Kind:     DiffChanged,
					OldValue: l.Address,
					NewValue: r.Address,
				})
			}
			entries = append(entries, diffFilterChains(base, l.FilterChains, r.FilterChains)...)
		}
	}

	return entries
}

func diffFilterChains(base string, left, right []FilterChain) []DiffEntry {
	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}

	var entries []DiffEntry
	for i := 0; i < maxLen; i++ {
		chainBase := fmt.Sprintf("%s/FilterChains[%d]", base, i)
		if i >= len(left) {
			entries = append(entries, DiffEntry{Path: chainBase, Kind: DiffAdded})
			continue
		}
		if i >= len(right) {
			entries = append(entries, DiffEntry{Path: chainBase, Kind: DiffRemoved})
			continue
		}
		entries = append(entries, diffHTTPFilters(chainBase, left[i].HttpFilters, right[i].HttpFilters)...)
	}
	return entries
}

func diffHTTPFilters(base string, left, right []HttpFilter) []DiffEntry {
	leftNames := filterNames(left)
	rightNames := filterNames(right)

	if strings.Join(leftNames, ",") == strings.Join(rightNames, ",") {
		return nil
	}

	leftSet := toSet(leftNames)
	rightSet := toSet(rightNames)

	var entries []DiffEntry
	maxLen := len(leftNames)
	if len(rightNames) > maxLen {
		maxLen = len(rightNames)
	}

	for i := 0; i < maxLen; i++ {
		path := fmt.Sprintf("%s/HttpFilters[%d]", base, i)
		switch {
		case i >= len(leftNames):
			entries = append(entries, DiffEntry{
				Path:     path,
				Kind:     DiffAdded,
				NewValue: rightNames[i],
			})
		case i >= len(rightNames):
			entries = append(entries, DiffEntry{
				Path:     path,
				Kind:     DiffRemoved,
				OldValue: leftNames[i],
			})
		default:
			if leftNames[i] != rightNames[i] {
				kind := DiffChanged
				if _, ok := rightSet[leftNames[i]]; !ok {
					kind = DiffRemoved
				} else if _, ok := leftSet[rightNames[i]]; !ok {
					kind = DiffAdded
				}
				entries = append(entries, DiffEntry{
					Path:     path,
					Kind:     kind,
					OldValue: leftNames[i],
					NewValue: rightNames[i],
				})
			}
		}
	}

	return entries
}

func diffRoutes(left, right []RouteConfig) []DiffEntry {
	leftByName := indexRoutes(left)
	rightByName := indexRoutes(right)

	var names []string
	seen := map[string]struct{}{}
	for name := range leftByName {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for name := range rightByName {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var entries []DiffEntry
	for _, name := range names {
		l, lok := leftByName[name]
		r, rok := rightByName[name]
		base := "Routes/" + name

		switch {
		case lok && !rok:
			entries = append(entries, DiffEntry{Path: base, Kind: DiffRemoved})
		case !lok && rok:
			entries = append(entries, DiffEntry{Path: base, Kind: DiffAdded})
		default:
			entries = append(entries, diffVirtualHosts(base, l.VirtualHosts, r.VirtualHosts)...)
		}
	}

	return entries
}

func diffVirtualHosts(base string, left, right []VirtualHost) []DiffEntry {
	leftByName := map[string]VirtualHost{}
	rightByName := map[string]VirtualHost{}
	for _, vhost := range left {
		leftByName[vhost.Name] = vhost
	}
	for _, vhost := range right {
		rightByName[vhost.Name] = vhost
	}

	var names []string
	seen := map[string]struct{}{}
	for name := range leftByName {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for name := range rightByName {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var entries []DiffEntry
	for _, name := range names {
		l, lok := leftByName[name]
		r, rok := rightByName[name]
		vhostBase := base + "/VirtualHosts/" + name

		switch {
		case lok && !rok:
			entries = append(entries, DiffEntry{Path: vhostBase, Kind: DiffRemoved})
		case !lok && rok:
			entries = append(entries, DiffEntry{Path: vhostBase, Kind: DiffAdded})
		default:
			if strings.Join(l.Domains, ",") != strings.Join(r.Domains, ",") {
				entries = append(entries, DiffEntry{
					Path:     vhostBase + "/Domains",
					Kind:     DiffChanged,
					OldValue: strings.Join(l.Domains, ", "),
					NewValue: strings.Join(r.Domains, ", "),
				})
			}
			entries = append(entries, diffRouteEntries(vhostBase, l.Routes, r.Routes)...)
		}
	}

	return entries
}

func diffRouteEntries(base string, left, right []Route) []DiffEntry {
	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}

	var entries []DiffEntry
	for i := 0; i < maxLen; i++ {
		routeBase := fmt.Sprintf("%s/Routes[%d]", base, i)
		switch {
		case i >= len(left):
			entries = append(entries, DiffEntry{
				Path:     routeBase,
				Kind:     DiffAdded,
				NewValue: formatRouteSummary(right[i]),
			})
		case i >= len(right):
			entries = append(entries, DiffEntry{
				Path:     routeBase,
				Kind:     DiffRemoved,
				OldValue: formatRouteSummary(left[i]),
			})
		default:
			l, r := left[i], right[i]
			if l.Match != r.Match {
				entries = append(entries, DiffEntry{
					Path:     routeBase + "/Match",
					Kind:     DiffChanged,
					OldValue: l.Match,
					NewValue: r.Match,
				})
			}
			if l.Cluster != r.Cluster {
				entries = append(entries, DiffEntry{
					Path:     routeBase + "/Cluster",
					Kind:     DiffChanged,
					OldValue: l.Cluster,
					NewValue: r.Cluster,
				})
			}
			if l.Timeout != r.Timeout {
				entries = append(entries, DiffEntry{
					Path:     routeBase + "/Timeout",
					Kind:     DiffChanged,
					OldValue: l.Timeout,
					NewValue: r.Timeout,
				})
			}
		}
	}

	return entries
}

func formatRouteSummary(route Route) string {
	parts := []string{route.Match}
	if route.Cluster != "" {
		parts = append(parts, "->"+route.Cluster)
	}
	if route.Timeout != "" {
		parts = append(parts, "timeout="+route.Timeout)
	}
	return strings.Join(parts, " ")
}

func indexRoutes(routes []RouteConfig) map[string]RouteConfig {
	result := make(map[string]RouteConfig, len(routes))
	for _, route := range routes {
		result[route.Name] = route
	}
	return result
}

func diffClusters(left, right []Cluster) []DiffEntry {
	leftByName := indexClusters(left)
	rightByName := indexClusters(right)

	var names []string
	seen := map[string]struct{}{}
	for name := range leftByName {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for name := range rightByName {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var entries []DiffEntry
	for _, name := range names {
		l, lok := leftByName[name]
		r, rok := rightByName[name]
		base := "Clusters/" + name

		switch {
		case lok && !rok:
			entries = append(entries, DiffEntry{Path: base, Kind: DiffRemoved, OldValue: l.Type})
		case !lok && rok:
			entries = append(entries, DiffEntry{Path: base, Kind: DiffAdded, NewValue: r.Type})
		default:
			if l.Type != r.Type {
				entries = append(entries, DiffEntry{
					Path:     base + "/Type",
					Kind:     DiffChanged,
					OldValue: l.Type,
					NewValue: r.Type,
				})
			}
			if l.ConnectTimeout != r.ConnectTimeout {
				entries = append(entries, DiffEntry{
					Path:     base + "/ConnectTimeout",
					Kind:     DiffChanged,
					OldValue: l.ConnectTimeout,
					NewValue: r.ConnectTimeout,
				})
			}
			if !equalStringSets(l.Endpoints, r.Endpoints) {
				entries = append(entries, DiffEntry{
					Path:     base + "/Endpoints",
					Kind:     DiffChanged,
					OldValue: strings.Join(l.Endpoints, ", "),
					NewValue: strings.Join(r.Endpoints, ", "),
				})
			}
		}
	}

	return entries
}

func indexListeners(listeners []Listener) map[string]Listener {
	result := make(map[string]Listener, len(listeners))
	for _, listener := range listeners {
		result[listener.Name] = listener
	}
	return result
}

func indexClusters(clusters []Cluster) map[string]Cluster {
	result := make(map[string]Cluster, len(clusters))
	for _, cluster := range clusters {
		result[cluster.Name] = cluster
	}
	return result
}

func filterNames(filters []HttpFilter) []string {
	names := make([]string, len(filters))
	for i, filter := range filters {
		names[i] = filter.Name
	}
	return names
}

func toSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}
