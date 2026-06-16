package envoyviz

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeLaterFileWins(t *testing.T) {
	left := EnvoyConfig{
		Listeners: []Listener{{Name: "listener_a", Address: "0.0.0.0:8080"}},
	}
	right := EnvoyConfig{
		Listeners: []Listener{{Name: "listener_a", Address: "0.0.0.0:9090"}},
	}

	merged := Merge([]EnvoyConfig{left, right})
	if merged.Listeners[0].Address != "0.0.0.0:9090" {
		t.Fatalf("address = %q, want later file value", merged.Listeners[0].Address)
	}
}

func TestEmptyFolderErrors(t *testing.T) {
	dir := t.TempDir()
	_, err := ParsePath(dir, ParseOptions{})
	if err == nil {
		t.Fatal("expected error for empty folder")
	}
}

func TestScanSupportedFilesNonRecursive(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.json"), []byte(`{"configs":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.json"), []byte(`{"configs":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := scanSupportedFiles(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %v, want only top-level file", files)
	}
}
