package envoyviz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// ParseOptions controls path and stdin parsing.
type ParseOptions struct {
	Recursive bool
	Format    string // "json" or "yaml"; used for stdin when set
}

// ParsePath parses a file or folder path into a merged EnvoyConfig.
func ParsePath(path string, opts ParseOptions) (EnvoyConfig, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return EnvoyConfig{}, fmt.Errorf("read stdin: %w", err)
		}
		filename := "stdin.json"
		if opts.Format == "yaml" || opts.Format == "yml" {
			filename = "stdin.yaml"
		}
		return Parse(data, filename)
	}

	info, err := os.Stat(path)
	if err != nil {
		return EnvoyConfig{}, fmt.Errorf("stat %s: %w", path, err)
	}

	if info.IsDir() {
		files, err := scanSupportedFiles(path, opts.Recursive)
		if err != nil {
			return EnvoyConfig{}, err
		}
		if len(files) == 0 {
			return EnvoyConfig{}, fmt.Errorf("no supported config files in %s", path)
		}

		var configs []EnvoyConfig
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				return EnvoyConfig{}, fmt.Errorf("read %s: %w", file, err)
			}
			cfg, err := Parse(data, file)
			if err != nil {
				return EnvoyConfig{}, fmt.Errorf("parse %s: %w", file, err)
			}
			configs = append(configs, cfg)
		}
		return Merge(configs), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return EnvoyConfig{}, fmt.Errorf("read %s: %w", path, err)
	}
	return Parse(data, path)
}

// Parse decodes a single file's bytes into EnvoyConfig.
func Parse(data []byte, filename string) (EnvoyConfig, error) {
	root, err := decodeConfig(data, filename)
	if err != nil {
		return EnvoyConfig{}, err
	}
	return parseRoot(root)
}

// Merge combines multiple EnvoyConfig values; later entries win on name collision.
func Merge(configs []EnvoyConfig) EnvoyConfig {
	if len(configs) == 0 {
		return EnvoyConfig{}
	}

	merged := EnvoyConfig{Version: configs[len(configs)-1].Version}
	listenerByName := map[string]Listener{}
	routeByName := map[string]RouteConfig{}
	clusterByName := map[string]Cluster{}

	for _, cfg := range configs {
		if cfg.Version != "" {
			merged.Version = cfg.Version
		}
		for _, listener := range cfg.Listeners {
			listenerByName[listener.Name] = listener
		}
		for _, route := range cfg.Routes {
			routeByName[route.Name] = route
		}
		for _, cluster := range cfg.Clusters {
			clusterByName[cluster.Name] = cluster
		}
	}

	merged.Listeners = mapValuesSorted(listenerByName, func(l Listener) string { return l.Name })
	merged.Routes = mapValuesSorted(routeByName, func(r RouteConfig) string { return r.Name })
	merged.Clusters = mapValuesSorted(clusterByName, func(c Cluster) string { return c.Name })
	return merged
}

func scanSupportedFiles(dir string, recursive bool) ([]string, error) {
	var files []string

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != dir && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if isSupportedConfigFile(path) {
			files = append(files, path)
		}
		return nil
	}

	if err := filepath.Walk(dir, walkFn); err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}

	sort.Strings(files)
	return files, nil
}

func isSupportedConfigFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func decodeConfig(data []byte, filename string) (map[string]any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty config input")
	}

	format := detectFormat(trimmed, filename)
	switch format {
	case "json":
		var root map[string]any
		if err := json.Unmarshal(trimmed, &root); err != nil {
			return nil, fmt.Errorf("decode json: %w", err)
		}
		return root, nil
	case "yaml":
		var root map[string]any
		if err := yaml.Unmarshal(trimmed, &root); err != nil {
			return nil, fmt.Errorf("decode yaml: %w", err)
		}
		return root, nil
	default:
		return nil, fmt.Errorf("unsupported format for %s", filename)
	}
}

func detectFormat(data []byte, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	}
	if len(data) > 0 && data[0] == '{' {
		return "json"
	}
	return "yaml"
}

// mapValuesSorted sorts a map by its keys and returns a slice of values.
// used for sorting the listeners, routes, and clusters by name to ensure consistent ordering.
func mapValuesSorted[T any](items map[string]T, key func(T) string) []T {
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]T, 0, len(keys))
	for _, k := range keys {
		result = append(result, items[k])
	}
	return result
}
