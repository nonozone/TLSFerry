package main

import (
	"reflect"
	"testing"
)

func TestValidateVersion(t *testing.T) {
	for _, version := range []string{"v0.1.0", "v0.1.0-rc.1", "v12.34.56-beta.2"} {
		if err := validateVersion(version); err != nil {
			t.Errorf("validateVersion(%q): %v", version, err)
		}
	}

	for _, version := range []string{"", "0.1.0", "v0.1", "main", "v0.1.0 dirty"} {
		if err := validateVersion(version); err == nil {
			t.Errorf("validateVersion(%q) unexpectedly succeeded", version)
		}
	}
}

func TestParseGitStatusRejectsTrackedChangesAndSensitiveUntrackedFiles(t *testing.T) {
	status := " M README.md\n?? notes.txt\n?? secret.pem\n?? nested/config.json\n?? \"private key.p12\"\n"
	got := parseGitStatus(status)
	want := []string{
		"sensitive untracked file: nested/config.json",
		"sensitive untracked file: private key.p12",
		"sensitive untracked file: secret.pem",
		"tracked worktree change: README.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseGitStatus() = %#v, want %#v", got, want)
	}
}

func TestSensitiveTrackedPath(t *testing.T) {
	tests := map[string]bool{
		"config.example.json":        false,
		"config.json":                true,
		"nested/config.json":         true,
		"certificates/site.pem":      true,
		"certificates/site.key":      true,
		"certificates/site.p12":      true,
		".tlsferry/state.json":       true,
		"dist/tlsferry_linux.tar.gz": true,
		"internal/config/config.go":  false,
	}
	for path, want := range tests {
		if got := sensitiveTrackedPath(path); got != want {
			t.Errorf("sensitiveTrackedPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestForbiddenCloudPath(t *testing.T) {
	tests := map[string]bool{
		"cloud":                         true,
		"cloud/worker.ts":               true,
		"apps/cloud-console/index.tsx":  true,
		"internal/billing/service.go":   true,
		"migrations/001.sql":            true,
		"wrangler.toml":                 true,
		"docs/edition-boundary.md":      false,
		"internal/acmeissuer/client.go": false,
	}
	for path, want := range tests {
		if got := forbiddenCloudPath(path); got != want {
			t.Errorf("forbiddenCloudPath(%q) = %v, want %v", path, got, want)
		}
	}
}
