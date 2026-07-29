package edition

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCommunityRepositoryExcludesCloudPrivateUnits(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve edition boundary test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	forbidden := []string{
		"cloud",
		filepath.Join("apps", "cloud-console"),
		filepath.Join("internal", "account"),
		filepath.Join("internal", "billing"),
		filepath.Join("internal", "controlplane"),
		filepath.Join("internal", "tenant"),
		"migrations",
		"wrangler.toml",
	}
	for _, relativePath := range forbidden {
		path := filepath.Join(repositoryRoot, relativePath)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("Cloud-private path must not exist in TLSFerry CE: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect %s: %v", relativePath, err)
		}
	}
}
