package main

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	input := strings.NewReader(strings.Repeat("a", 64) + "  tlsferry_1.0.0_linux_amd64.tar.gz\n")
	got, err := parseChecksums(input)
	if err != nil {
		t.Fatalf("parseChecksums() error = %v", err)
	}
	if got["tlsferry_1.0.0_linux_amd64.tar.gz"] != strings.Repeat("a", 64) {
		t.Fatalf("parseChecksums() = %#v", got)
	}
}

func TestParseChecksumsRejectsUnsafeAndDuplicateNames(t *testing.T) {
	for _, input := range []string{
		strings.Repeat("a", 64) + "  ../archive.tar.gz\n",
		strings.Repeat("a", 64) + "  archive.tar.gz\n" + strings.Repeat("b", 64) + "  archive.tar.gz\n",
		"not-a-hash  archive.tar.gz\n",
	} {
		if _, err := parseChecksums(strings.NewReader(input)); err == nil {
			t.Fatalf("parseChecksums(%q) unexpectedly succeeded", input)
		}
	}
}

func TestExpectedArchiveNames(t *testing.T) {
	got := expectedArchiveNames("0.1.0-rc.1")
	want := []string{
		"tlsferry_0.1.0-rc.1_darwin_amd64.tar.gz",
		"tlsferry_0.1.0-rc.1_darwin_arm64.tar.gz",
		"tlsferry_0.1.0-rc.1_linux_amd64.tar.gz",
		"tlsferry_0.1.0-rc.1_linux_arm64.tar.gz",
		"tlsferry_0.1.0-rc.1_windows_amd64.zip",
		"tlsferry_0.1.0-rc.1_windows_arm64.zip",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expectedArchiveNames() = %#v, want %#v", got, want)
	}
}

func TestValidateArchiveEntries(t *testing.T) {
	source := map[string][]byte{
		"LICENSE":                           []byte("license"),
		"README.md":                         []byte("readme"),
		"config.example.json":               []byte("config"),
		"config.release-smoke.example.json": []byte("smoke config"),
	}
	entries := map[string]archiveEntry{
		"LICENSE":                           {Data: []byte("license")},
		"README.md":                         {Data: []byte("readme")},
		"config.example.json":               {Data: []byte("config")},
		"config.release-smoke.example.json": {Data: []byte("smoke config")},
		"tlsferry":                          {Data: []byte("binary"), Mode: 0o755},
	}
	binary, err := validateArchiveEntries(entries, "tlsferry", source, true)
	if err != nil || string(binary) != "binary" {
		t.Fatalf("validateArchiveEntries() = %q, %v", binary, err)
	}

	for name, mutate := range map[string]func(map[string]archiveEntry){
		"missing public file": func(value map[string]archiveEntry) {
			delete(value, "config.release-smoke.example.json")
		},
		"extra file": func(value map[string]archiveEntry) { value["private.key"] = archiveEntry{Data: []byte("secret")} },
		"changed public file": func(value map[string]archiveEntry) {
			value["config.release-smoke.example.json"] = archiveEntry{Data: []byte("changed")}
		},
		"non-executable binary": func(value map[string]archiveEntry) {
			value["tlsferry"] = archiveEntry{Data: []byte("binary"), Mode: fs.FileMode(0o644)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			copyEntries := make(map[string]archiveEntry, len(entries))
			for key, value := range entries {
				copyEntries[key] = value
			}
			mutate(copyEntries)
			if _, err := validateArchiveEntries(copyEntries, "tlsferry", source, true); err == nil {
				t.Fatal("validateArchiveEntries() unexpectedly succeeded")
			}
		})
	}
}
