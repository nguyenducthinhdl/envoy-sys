package envoyviz

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorDim    = "\033[2m"
)

// RenderOptions controls terminal output.
type RenderOptions struct {
	NoColor   bool
	LeftLabel string
	RightLabel string
}

// FormatConfig renders a parsed config as a text tree.
func FormatConfig(cfg EnvoyConfig) string {
	var b strings.Builder
	if cfg.Version != "" {
		b.WriteString("Version: " + cfg.Version + "\n")
	}

	b.WriteString("Listeners\n")
	if len(cfg.Listeners) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, listener := range cfg.Listeners {
			writeListenerTree(&b, listener, "  ")
		}
	}

	b.WriteString("Routes\n")
	if len(cfg.Routes) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, route := range cfg.Routes {
			writeRouteTree(&b, route, cfg.Clusters, "  ")
		}
	}

	b.WriteString("Clusters\n")
	if len(cfg.Clusters) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, cluster := range cfg.Clusters {
			writeClusterTree(&b, cluster, "  ")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// RenderDiff prints a side-by-side tree diff to w.
func RenderDiff(w io.Writer, left, right EnvoyConfig, diff DiffResult, opts RenderOptions) {
	useColor := !opts.NoColor && isTerminal(w)
	leftLabel := defaultLabel(opts.LeftLabel, "Left")
	rightLabel := defaultLabel(opts.RightLabel, "Right")

	diffByPath := map[string]DiffEntry{}
	for _, entry := range diff.Entries {
		diffByPath[entry.Path] = entry
	}

	rows := alignConfigRows(left, right)

	leftWidth := 34
	for _, row := range rows {
		if len(row.left) > leftWidth {
			leftWidth = len(row.left)
		}
	}

	fmt.Fprintf(w, "%s │ %s\n", padRight(leftLabel, leftWidth), rightLabel)

	for _, row := range rows {
		entry, hasDiff := diffForRow(row.leftPath, row.rightPath, diffByPath)
		leftCol := colorizeValue(row.left, entry, true, useColor, hasDiff)
		rightCol := colorizeValue(row.right, entry, false, useColor, hasDiff)

		if leftCol == "" && rightCol == "" {
			continue
		}

		fmt.Fprintf(w, "%s │ %s\n", padRight(leftCol, leftWidth+colorOverhead(leftCol, row.left)), rightCol)
	}
}

type configRow struct {
	text string
	path string
}

type alignedRow struct {
	left, right       string
	leftPath, rightPath string
}

func alignConfigRows(left, right EnvoyConfig) []alignedRow {
	leftRows := buildConfigRows(left, "Listeners", "Routes", "Clusters")
	rightRows := buildConfigRows(right, "Listeners", "Routes", "Clusters")

	leftByPath := map[string]configRow{}
	rightByPath := map[string]configRow{}
	var order []string
	seen := map[string]struct{}{}

	appendOrder := func(path string) {
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		order = append(order, path)
	}

	for _, row := range leftRows {
		leftByPath[row.path] = row
		appendOrder(row.path)
	}
	for _, row := range rightRows {
		rightByPath[row.path] = row
		appendOrder(row.path)
	}

	rows := make([]alignedRow, 0, len(order))
	for _, path := range order {
		l, lok := leftByPath[path]
		r, rok := rightByPath[path]
		row := alignedRow{leftPath: path, rightPath: path}
		if lok {
			row.left = l.text
		}
		if rok {
			row.right = r.text
		}
		rows = append(rows, row)
	}
	return rows
}

func buildConfigRows(cfg EnvoyConfig, sections ...string) []configRow {
	var rows []configRow
	for _, section := range sections {
		rows = append(rows, configRow{text: section, path: section})
		switch section {
		case "Listeners":
			for _, listener := range cfg.Listeners {
				rows = append(rows, listenerRows(listener)...)
			}
		case "Routes":
			for _, route := range cfg.Routes {
				rows = append(rows, routeRows(route, cfg.Clusters)...)
			}
		case "Clusters":
			for _, cluster := range cfg.Clusters {
				rows = append(rows, clusterRows(cluster)...)
			}
		}
	}
	return rows
}

func listenerRows(listener Listener) []configRow {
	base := "Listeners/" + listener.Name
	rows := []configRow{
		{text: "  " + listener.Name, path: base},
		{text: "    Address: " + listener.Address, path: base + "/Address"},
	}
	for i, chain := range listener.FilterChains {
		chainBase := fmt.Sprintf("%s/FilterChains[%d]", base, i)
		rows = append(rows, configRow{text: fmt.Sprintf("    FilterChains[%d]", i), path: chainBase})
		if chain.RouteConfigName != "" {
			rows = append(rows, configRow{
				text: fmt.Sprintf("      RouteConfig: %s", chain.RouteConfigName),
				path: chainBase + "/RouteConfigName",
			})
		}
		rows = append(rows, configRow{text: "      HttpFilters", path: chainBase + "/HttpFilters"})
		for j, filter := range chain.HttpFilters {
			rows = append(rows, configRow{
				text: fmt.Sprintf("        %s", filter.Name),
				path: fmt.Sprintf("%s/HttpFilters[%d]", chainBase, j),
			})
		}
	}
	return rows
}

func routeRows(route RouteConfig, clusters []Cluster) []configRow {
	base := "Routes/" + route.Name
	rows := []configRow{{text: "  " + route.Name, path: base}}

	for _, vhost := range route.VirtualHosts {
		vhostBase := base + "/VirtualHosts/" + vhost.Name
		rows = append(rows, configRow{
			text: fmt.Sprintf("    VirtualHost: %s", vhost.Name),
			path: vhostBase,
		})
		if len(vhost.Domains) > 0 {
			rows = append(rows, configRow{
				text: fmt.Sprintf("      Domains: %s", strings.Join(vhost.Domains, ", ")),
				path: vhostBase + "/Domains",
			})
		}
		for i, entry := range vhost.Routes {
			routeBase := fmt.Sprintf("%s/Routes[%d]", vhostBase, i)
			rows = append(rows, configRow{
				text: fmt.Sprintf("      %s -> %s", entry.Match, entry.Cluster),
				path: routeBase + "/Cluster",
			})
			if entry.Timeout != "" {
				rows = append(rows, configRow{
					text: fmt.Sprintf("        Timeout: %s", entry.Timeout),
					path: routeBase + "/Timeout",
				})
			}
			if dest := formatDestination(entry.Cluster, clusters); dest != "" {
				rows = append(rows, configRow{
					text: fmt.Sprintf("        Destination: %s", dest),
					path: routeBase + "/Destination",
				})
			}
		}
	}

	return rows
}

func clusterRows(cluster Cluster) []configRow {
	base := "Clusters/" + cluster.Name
	rows := []configRow{
		{text: "  " + cluster.Name, path: base},
		{text: "    Type: " + cluster.Type, path: base + "/Type"},
		{text: "    ConnectTimeout: " + cluster.ConnectTimeout, path: base + "/ConnectTimeout"},
	}
	if len(cluster.Endpoints) > 0 {
		rows = append(rows, configRow{
			text: "    Endpoints: " + strings.Join(cluster.Endpoints, ", "),
			path: base + "/Endpoints",
		})
	}
	return rows
}

func diffForRow(leftPath, rightPath string, diffByPath map[string]DiffEntry) (DiffEntry, bool) {
	if entry, ok := diffByPath[leftPath]; ok {
		return entry, true
	}
	if entry, ok := diffByPath[rightPath]; ok {
		return entry, true
	}
	return DiffEntry{}, false
}

func colorizeValue(text string, entry DiffEntry, isLeft, useColor, hasDiff bool) string {
	if text == "" {
		return ""
	}
	if !useColor || !hasDiff {
		return text
	}

	switch entry.Kind {
	case DiffAdded:
		if !isLeft {
			return colorGreen + text + colorReset
		}
	case DiffRemoved:
		if isLeft {
			return colorRed + text + colorReset
		}
	case DiffChanged:
		return colorYellow + text + colorReset
	}
	return text
}

func writeListenerTree(b *strings.Builder, listener Listener, indent string) {
	b.WriteString(fmt.Sprintf("%s%s\n", indent, listener.Name))
	b.WriteString(fmt.Sprintf("%s  Address: %s\n", indent, listener.Address))
	for i, chain := range listener.FilterChains {
		b.WriteString(fmt.Sprintf("%s  FilterChains[%d]\n", indent, i))
		if chain.RouteConfigName != "" {
			b.WriteString(fmt.Sprintf("%s    RouteConfig: %s\n", indent, chain.RouteConfigName))
		}
		if len(chain.NetworkFilters) > 0 {
			b.WriteString(fmt.Sprintf("%s    NetworkFilters: %s\n", indent, strings.Join(chain.NetworkFilters, ", ")))
		}
		b.WriteString(fmt.Sprintf("%s    HttpFilters\n", indent))
		for _, filter := range chain.HttpFilters {
			b.WriteString(fmt.Sprintf("%s      %s\n", indent, filter.Name))
		}
	}
}

func writeRouteTree(b *strings.Builder, route RouteConfig, clusters []Cluster, indent string) {
	b.WriteString(fmt.Sprintf("%s%s\n", indent, route.Name))
	for _, vhost := range route.VirtualHosts {
		b.WriteString(fmt.Sprintf("%s  VirtualHost: %s\n", indent, vhost.Name))
		if len(vhost.Domains) > 0 {
			b.WriteString(fmt.Sprintf("%s    Domains: %s\n", indent, strings.Join(vhost.Domains, ", ")))
		}
		for _, entry := range vhost.Routes {
			b.WriteString(fmt.Sprintf("%s    %s -> %s\n", indent, entry.Match, entry.Cluster))
			if entry.Timeout != "" {
				b.WriteString(fmt.Sprintf("%s      Timeout: %s\n", indent, entry.Timeout))
			}
			if dest := formatDestination(entry.Cluster, clusters); dest != "" {
				b.WriteString(fmt.Sprintf("%s      Destination: %s\n", indent, dest))
			}
		}
	}
}

func formatDestination(clusterName string, clusters []Cluster) string {
	if clusterName == "" {
		return ""
	}
	for _, cluster := range clusters {
		if cluster.Name != clusterName {
			continue
		}
		if len(cluster.Endpoints) == 0 {
			return clusterName
		}
		return clusterName + " (" + strings.Join(cluster.Endpoints, ", ") + ")"
	}
	return clusterName
}

func writeClusterTree(b *strings.Builder, cluster Cluster, indent string) {
	b.WriteString(fmt.Sprintf("%s%s\n", indent, cluster.Name))
	b.WriteString(fmt.Sprintf("%s  Type: %s\n", indent, cluster.Type))
	if cluster.ConnectTimeout != "" {
		b.WriteString(fmt.Sprintf("%s  ConnectTimeout: %s\n", indent, cluster.ConnectTimeout))
	}
	if len(cluster.Endpoints) > 0 {
		b.WriteString(fmt.Sprintf("%s  Endpoints: %s\n", indent, strings.Join(cluster.Endpoints, ", ")))
	}
}

func defaultLabel(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func padRight(text string, width int) string {
	visible := visibleLength(text)
	if visible >= width {
		return text
	}
	return text + strings.Repeat(" ", width-visible)
}

func visibleLength(text string) int {
	clean := strings.ReplaceAll(text, colorReset, "")
	clean = strings.ReplaceAll(clean, colorGreen, "")
	clean = strings.ReplaceAll(clean, colorRed, "")
	clean = strings.ReplaceAll(clean, colorYellow, "")
	clean = strings.ReplaceAll(clean, colorDim, "")
	return len(clean)
}

func colorOverhead(colored, plain string) int {
	if colored == plain {
		return 0
	}
	return visibleLength(colored) - len(plain)
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
