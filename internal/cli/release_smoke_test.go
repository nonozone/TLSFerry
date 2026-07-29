//go:build release_smoke

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
	"github.com/nonozone/TLSFerry/internal/discovery"
)

func TestReleaseFunctionalSmoke(t *testing.T) {
	const (
		dnsToken         = "functional-smoke-dns-token"
		cloudSecretID    = "functional-smoke-secret-id"
		cloudSecretKey   = "functional-smoke-secret-key"
		selectedDomain   = "nos.example.invalid"
		unselectedDomain = "other.example.invalid"
	)
	t.Setenv("SMOKE_CF_API_TOKEN", dnsToken)
	t.Setenv("SMOKE_TENCENT_SECRET_ID", cloudSecretID)
	t.Setenv("SMOKE_TENCENT_SECRET_KEY", cloudSecretKey)
	secrets := []string{dnsToken, cloudSecretID, cloudSecretKey}
	redact := func(value string) string {
		for _, secret := range secrets {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
		return value
	}

	configPath := filepath.Join(t.TempDir(), "config.json")
	original := config.Config{
		RenewBefore: "720h",
		Certificates: []config.Certificate{{
			Name:    "existing",
			Domains: []string{"assets.example.invalid"},
			Issuer: config.Issuer{
				Type:         "acme",
				Email:        "ops@example.invalid",
				DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory",
				Challenge:    "dns-01",
				DNSProvider:  "cloudflare",
				Credential:   "env:SMOKE_CF",
			},
			Deployments: []config.Deployment{{
				Provider:   "tencent-cdn",
				Target:     "assets.example.invalid",
				Credential: "env:SMOKE_TENCENT",
			}},
		}},
	}
	if err := config.Save(configPath, original); err != nil {
		t.Fatalf("save smoke configuration: %v", err)
	}

	originalScanner := newCloudScanner
	scannerCalls := 0
	newCloudScanner = func(provider string, _ credential.Resolver, reference string) (discovery.Scanner, error) {
		scannerCalls++
		if provider != "tencent" {
			t.Fatalf("scanner provider = %q, want tencent", provider)
		}
		if reference != "env:SMOKE_TENCENT" {
			t.Fatalf("scanner credential = %q, want env:SMOKE_TENCENT", reference)
		}
		return releaseSmokeScanner{domains: []discovery.Domain{
			{Provider: "tencent", Name: selectedDomain, Status: "online", HTTPS: false, CNAME: "nos.cdn.example.invalid"},
			{Provider: "tencent", Name: unselectedDomain, Status: "online", HTTPS: true, CNAME: "other.cdn.example.invalid"},
		}}, nil
	}
	t.Cleanup(func() { newCloudScanner = originalScanner })

	var transcript strings.Builder
	run := func(name string, args ...string) string {
		t.Helper()
		var stdout, stderr strings.Builder
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%s code = %d, stdout = %q, stderr = %q", name, code, redact(stdout.String()), redact(stderr.String()))
		}
		transcript.WriteString(stdout.String())
		transcript.WriteString(stderr.String())
		return stdout.String()
	}
	readConfig := func() []byte {
		t.Helper()
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read smoke configuration: %v", err)
		}
		return content
	}
	assertReadOnly := func(name string, args ...string) string {
		t.Helper()
		before := readConfig()
		stdout := run(name, args...)
		after := readConfig()
		if !bytes.Equal(before, after) {
			t.Fatalf("%s changed the configuration", name)
		}
		return stdout
	}

	if output := assertReadOnly("validate", "validate", "--config", configPath); !strings.Contains(output, "configuration is valid") {
		t.Fatalf("validate output = %q", output)
	}
	if output := assertReadOnly("plan", "plan", "--config", configPath); !strings.Contains(output, "assets.example.invalid") {
		t.Fatalf("plan output = %q", output)
	}
	if output := assertReadOnly("preflight", "preflight", "--config", configPath); !strings.Contains(output, "preflight passed") {
		t.Fatalf("preflight output = %q", output)
	}
	if output := assertReadOnly(
		"discover",
		"discover", "cloud", "--provider", "tencent", "--credential", "env:SMOKE_TENCENT", "--format", "json",
	); !strings.Contains(output, selectedDomain) || !strings.Contains(output, unselectedDomain) {
		t.Fatalf("discover output = %q", output)
	}

	enrollArgs := []string{
		"enroll", "cloud", "--provider", "tencent", "--domain", selectedDomain,
		"--email", "ops@example.invalid", "--dns-provider", "cloudflare",
		"--dns-credential", "env:SMOKE_CF", "--credential", "env:SMOKE_TENCENT",
		"--directory-url", "https://acme-staging-v02.api.letsencrypt.org/directory",
		"--config", configPath,
	}
	if output := assertReadOnly("enroll preview", enrollArgs...); !strings.Contains(output, "No changes made") {
		t.Fatalf("enroll preview output = %q", output)
	}
	run("enroll execute", append(enrollArgs, "--execute")...)

	updated, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load enrolled configuration: %v", err)
	}
	if len(updated.Certificates) != 2 {
		t.Fatalf("enrolled certificate count = %d, want 2", len(updated.Certificates))
	}
	if !reflect.DeepEqual(updated.Certificates[0], original.Certificates[0]) {
		t.Fatalf("existing certificate changed: %#v", updated.Certificates[0])
	}
	added := updated.Certificates[1]
	if len(added.Domains) != 1 || added.Domains[0] != selectedDomain || added.Deployments[0].Target != selectedDomain {
		t.Fatalf("selected enrollment = %#v", added)
	}
	if strings.Contains(string(readConfig()), unselectedDomain) {
		t.Fatalf("unselected inventory domain %q was written", unselectedDomain)
	}
	assertReadOnly("post-enrollment preflight", "preflight", "--config", configPath)

	if scannerCalls != 3 {
		t.Fatalf("scanner calls = %d, want 3", scannerCalls)
	}
	for index, secret := range secrets {
		if strings.Contains(transcript.String(), secret) {
			t.Fatalf("functional smoke output exposed credential value %d", index+1)
		}
	}
}

type releaseSmokeScanner struct {
	domains []discovery.Domain
}

func (s releaseSmokeScanner) Scan(context.Context) ([]discovery.Domain, error) {
	return append([]discovery.Domain(nil), s.domains...), nil
}
