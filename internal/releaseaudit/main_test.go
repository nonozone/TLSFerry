package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
		"config.example.json":               false,
		"config.release-smoke.example.json": false,
		"config.json":                       true,
		"nested/config.json":                true,
		"certificates/site.pem":             true,
		"certificates/site.key":             true,
		"certificates/site.p12":             true,
		".tlsferry/state.json":              true,
		"dist/tlsferry_linux.tar.gz":        true,
		"internal/config/config.go":         false,
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

func TestAuditReleaseWorkflowPermissions(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workflow := `name: Release CE
permissions: {}
jobs:
  verify:
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
        with:
          persist-credentials: false
  publish:
    needs: verify
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
        with:
          persist-credentials: false
      - uses: goreleaser/goreleaser-action@e435ccd777264be153ace6237001ef4d979d3a7a
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
`
	workflowPath := filepath.Join(workflowDir, "release.yml")
	if err := os.WriteFile(workflowPath, []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}
	if failures := auditReleaseWorkflowPermissions(root); len(failures) != 0 {
		t.Fatalf("secure workflow failures = %v", failures)
	}

	insecure := strings.Replace(workflow, "permissions: {}", "permissions:\n  contents: write", 1)
	insecure = strings.Replace(insecure, "          persist-credentials: false\n", "", 1)
	if err := os.WriteFile(workflowPath, []byte(insecure), 0o644); err != nil {
		t.Fatal(err)
	}
	failures := auditReleaseWorkflowPermissions(root)
	if len(failures) == 0 {
		t.Fatal("insecure release workflow unexpectedly passed")
	}
}
