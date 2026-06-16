package envoyviz

import (
	"strings"
	"testing"
)

func TestDiffConfigDump1And2(t *testing.T) {
	left := mustParseFile(t, "testdata/config-dump-1.json")
	right := mustParseFile(t, "testdata/config-dump-2.json")

	result := Compare(left, right)
	if !result.HasDiffs() {
		t.Fatal("expected differences")
	}

	text := diffText(result)
	if !strings.Contains(text, "ConnectTimeout") || !strings.Contains(text, "5s") || !strings.Contains(text, "2s") {
		t.Fatalf("expected connect_timeout diff, got: %s", text)
	}
	if !strings.Contains(text, "jwt_authn") || !strings.Contains(text, "rbac") {
		t.Fatalf("expected filter reorder diff, got: %s", text)
	}
}

func TestDiffIdenticalConfigs(t *testing.T) {
	left := mustParseFile(t, "testdata/config-dump-1.json")
	right := mustParseFile(t, "testdata/config-dump-1.json")

	result := Compare(left, right)
	if result.HasDiffs() {
		t.Fatalf("expected no diffs, got %v", result.Entries)
	}
}

func diffText(result DiffResult) string {
	var b strings.Builder
	for _, entry := range result.Entries {
		b.WriteString(entry.Path)
		b.WriteString(" ")
		b.WriteString(entry.OldValue)
		b.WriteString(" ")
		b.WriteString(entry.NewValue)
		b.WriteString("\n")
	}
	return b.String()
}
