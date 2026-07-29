package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
	"github.com/nonozone/TLSFerry/internal/discovery"
)

func TestReleaseSmokeRefusesProductionACME(t *testing.T) {
	configPath := saveReleaseSmokeConfig(t, "https://acme-v02.api.letsencrypt.org/directory")
	var stdout, stderr strings.Builder
	code := Run([]string{"release-smoke", "--config", configPath, "--certificate", "staging", "--provider", "tencent-cdn"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "production ACME is not allowed") {
		t.Fatalf("Run() code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
}

func TestReleaseSmokePreviewMakesNoExternalCalls(t *testing.T) {
	configPath := saveReleaseSmokeConfig(t, letsEncryptStagingDirectory)
	called := false
	originalIssue := releaseSmokeIssue
	releaseSmokeIssue = func([]string, io.Writer, io.Writer) int { called = true; return 0 }
	t.Cleanup(func() { releaseSmokeIssue = originalIssue })
	var stdout, stderr strings.Builder
	code := Run([]string{"release-smoke", "--config", configPath, "--certificate", "staging", "--provider", "tencent-cdn"}, &stdout, &stderr)
	if code != 0 || called || !strings.Contains(stdout.String(), "No external operations performed") {
		t.Fatalf("Run() code = %d, called = %t, stdout = %q, stderr = %q", code, called, stdout.String(), stderr.String())
	}
}

func TestReleaseSmokeRequiresCoveredAndExactlyConfirmedTarget(t *testing.T) {
	configPath := saveReleaseSmokeConfig(t, letsEncryptStagingDirectory)
	var stdout, stderr strings.Builder
	code := Run([]string{"release-smoke", "--config", configPath, "--certificate", "staging", "--provider", "tencent-cdn", "--confirm-test-target", "wrong.example.com", "--accept-tos", "--execute"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "must exactly equal configured target") {
		t.Fatalf("confirmation code = %d, stderr = %q", code, stderr.String())
	}

	mismatchedPath := filepath.Join(t.TempDir(), "config.json")
	err := config.Save(mismatchedPath, config.Config{RenewBefore: "720h", Certificates: []config.Certificate{{
		Name: "staging", Domains: []string{"other.example.com"},
		Issuer:      config.Issuer{Type: "acme", Email: "ops@example.com", DirectoryURL: letsEncryptStagingDirectory, Challenge: "dns-01", DNSProvider: "cloudflare", Credential: "env:CLOUDFLARE"},
		Deployments: []config.Deployment{{Provider: "tencent-cdn", Target: "staging.example.com", Credential: "env:TENCENTCLOUD"}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"release-smoke", "--config", mismatchedPath, "--certificate", "staging", "--provider", "tencent-cdn"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "domains do not cover deployment target") {
		t.Fatalf("coverage code = %d, stderr = %q", code, stderr.String())
	}
}

func TestReleaseSmokeExecutesExistingPathsAndWritesSanitizedPendingEvidence(t *testing.T) {
	configPath := saveReleaseSmokeConfig(t, letsEncryptStagingDirectory)
	evidencePath := filepath.Join(t.TempDir(), "evidence.json")
	originalPreflight, originalIssue, originalDeploy := releaseSmokePreflight, releaseSmokeIssue, releaseSmokeDeploy
	originalLoad, originalNow := releaseSmokeLoadBundle, releaseSmokeNow
	releaseSmokePreflight = func([]string, io.Writer, io.Writer) int { return 0 }
	releaseSmokeIssue = func([]string, io.Writer, io.Writer) int { return 0 }
	releaseSmokeDeploy = func(_ []string, stdout, _ io.Writer) int {
		fmt.Fprintln(stdout, "deployment submitted: tencent-cdn -> staging.example.com")
		fmt.Fprintln(stdout, "  reference: request-42")
		return 0
	}
	releaseSmokeLoadBundle = func(string, string) (certstore.Bundle, error) {
		return certstore.Bundle{Domains: []string{"staging.example.com"}, Certificate: []byte("public-certificate"), PrivateKey: []byte("never-write-this-secret"), IssuedAt: time.Date(2026, time.July, 29, 10, 0, 0, 0, time.UTC)}, nil
	}
	releaseSmokeNow = func() time.Time { return time.Date(2026, time.July, 29, 10, 1, 0, 0, time.UTC) }
	t.Cleanup(func() {
		releaseSmokePreflight, releaseSmokeIssue, releaseSmokeDeploy = originalPreflight, originalIssue, originalDeploy
		releaseSmokeLoadBundle, releaseSmokeNow = originalLoad, originalNow
	})
	var stdout, stderr strings.Builder
	code := Run([]string{"release-smoke", "--config", configPath, "--certificate", "staging", "--provider", "tencent-cdn", "--confirm-test-target", "staging.example.com", "--accept-tos", "--execute", "--evidence", evidencePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "never-write-this-secret") || !strings.Contains(string(data), `"gate_status": "pending_cleanup"`) || !strings.Contains(string(data), `"reference": "request-42"`) {
		t.Fatalf("evidence = %s", data)
	}
	var evidence releaseSmokeEvidence
	if err := json.Unmarshal(data, &evidence); err != nil || evidence.Certificate.PublicSHA256 == "" {
		t.Fatalf("decode evidence: %v, value = %#v", err, evidence)
	}
}

func TestReleaseSmokeCleanupPreservesOriginalAndCreatesReviewRecord(t *testing.T) {
	root := t.TempDir()
	pendingPath := filepath.Join(root, "evidence.json")
	readyPath := filepath.Join(root, "evidence.ready.json")
	evidence := releaseSmokeEvidence{SchemaVersion: 1, GateStatus: "pending_cleanup"}
	evidence.Deployment.Target = "staging.example.com"
	evidence.Cleanup.Status = "pending"
	if err := writeReleaseSmokeEvidence(pendingPath, evidence); err != nil {
		t.Fatal(err)
	}
	originalNow := releaseSmokeNow
	releaseSmokeNow = func() time.Time { return time.Date(2026, time.July, 29, 11, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { releaseSmokeNow = originalNow })
	var stdout, stderr strings.Builder
	code := Run([]string{"release-smoke", "cleanup", "--evidence", pendingPath, "--output", readyPath, "--confirm-test-target", "staging.example.com", "--cleanup-reference", "ticket/cleanup-42"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	pendingBytes, err := os.ReadFile(pendingPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pendingBytes), `"gate_status": "pending_cleanup"`) {
		t.Fatalf("pending evidence changed: %s", pendingBytes)
	}
	readyBytes, err := os.ReadFile(readyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readyBytes), `"gate_status": "ready_for_review"`) || !strings.Contains(string(readyBytes), `"status": "operator_confirmed"`) || !strings.Contains(string(readyBytes), `"reference": "ticket/cleanup-42"`) {
		t.Fatalf("ready evidence = %s", readyBytes)
	}
}

func TestReleaseSmokeCleanupRejectsWrongTargetUnsafeReferenceAndOverwrite(t *testing.T) {
	root := t.TempDir()
	pendingPath := filepath.Join(root, "evidence.json")
	evidence := releaseSmokeEvidence{SchemaVersion: 1, GateStatus: "pending_cleanup"}
	evidence.Deployment.Target = "staging.example.com"
	evidence.Cleanup.Status = "pending"
	if err := writeReleaseSmokeEvidence(pendingPath, evidence); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"wrong target", []string{"--confirm-test-target", "wrong.example.com", "--cleanup-reference", "request-1"}, "must exactly equal evidence target"},
		{"unsafe reference", []string{"--confirm-test-target", "staging.example.com", "--cleanup-reference", "secret value with spaces"}, "safe reference characters"},
		{"overwrite", []string{"--output", pendingPath, "--confirm-test-target", "staging.example.com", "--cleanup-reference", "request-1"}, "original record is preserved"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			args := append([]string{"release-smoke", "cleanup", "--evidence", pendingPath}, test.args...)
			if code := Run(args, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("Run() code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func saveReleaseSmokeConfig(t *testing.T, directoryURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	err := config.Save(path, config.Config{RenewBefore: "720h", Certificates: []config.Certificate{{
		Name: "staging", Domains: []string{"staging.example.com"},
		Issuer:      config.Issuer{Type: "acme", Email: "ops@example.com", DirectoryURL: directoryURL, Challenge: "dns-01", DNSProvider: "cloudflare", Credential: "env:CLOUDFLARE"},
		Deployments: []config.Deployment{{Provider: "tencent-cdn", Target: "staging.example.com", Credential: "env:TENCENTCLOUD"}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIssueRequiresCertificateName(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"issue", "--accept-tos"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--certificate is required") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestVersionPrintsBuildVersion(t *testing.T) {
	originalVersion := version
	version = "v1.2.3"
	t.Cleanup(func() { version = originalVersion })

	var stdout, stderr strings.Builder
	code := Run([]string{"version"}, &stdout, &stderr)
	if code != 0 || stdout.String() != "TLSFerry v1.2.3\n" || stderr.Len() != 0 {
		t.Fatalf("Run() code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
}

func TestIssueRequiresTermsAcceptance(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"issue", "--certificate", "assets"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--accept-tos is required") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestDeployRequiresProvider(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"deploy", "--certificate", "assets", "--execute"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--provider is required") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestDeployRequiresExplicitExecution(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"deploy", "--certificate", "assets", "--provider", "tencent-cdn"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--execute is required") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestRenewRequiresSafetyFlags(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := Run([]string{"renew"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "--accept-tos") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
	stderr.Reset()
	if code := Run([]string{"renew", "--accept-tos"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "--execute") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestAuthLoginStoresProviderCredentialInKeychainProfile(t *testing.T) {
	store := &fakeCredentialStore{}
	originalStore := newCredentialStore
	newCredentialStore = func() credential.Store { return store }
	t.Cleanup(func() { newCredentialStore = originalStore })

	var stdout, stderr strings.Builder
	stdin := bytes.NewBufferString("secret-id\nsecret-key\n")
	code := RunWithInput([]string{"auth", "login", "tencent", "--no-browser"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunWithInput() code = %d, stderr = %q", code, stderr.String())
	}
	if store.profile != "TENCENTCLOUD" || store.values["SECRET_ID"] != "secret-id" || store.values["SECRET_KEY"] != "secret-key" {
		t.Fatalf("stored profile = %q, values = %#v", store.profile, store.values)
	}
	if strings.Contains(stdout.String(), "secret-id") || strings.Contains(stdout.String(), "secret-key") {
		t.Fatalf("stdout exposed credential values: %q", stdout.String())
	}
}

func TestAuthLoginStoresCloudflareAPIToken(t *testing.T) {
	store := &fakeCredentialStore{}
	originalStore := newCredentialStore
	newCredentialStore = func() credential.Store { return store }
	t.Cleanup(func() { newCredentialStore = originalStore })

	var stdout, stderr strings.Builder
	stdin := bytes.NewBufferString("super-secret-value\n")
	code := RunWithInput([]string{"auth", "login", "cloudflare", "--no-browser"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunWithInput() code = %d, stderr = %q", code, stderr.String())
	}
	if store.profile != "CLOUDFLARE" || store.values["API_TOKEN"] != "super-secret-value" {
		t.Fatalf("stored profile = %q, values = %#v", store.profile, store.values)
	}
	if strings.Contains(stdout.String(), "super-secret-value") {
		t.Fatalf("stdout exposed credential values: %q", stdout.String())
	}
}

func TestServiceInstallRequiresExplicitSafetyFlags(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"service", "install"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--accept-tos and --execute are required") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestDiscoverCloudRequiresProvider(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"discover", "cloud"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--provider is required") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestEnrollCloudPreviewsThenWritesSelectedDomain(t *testing.T) {
	originalScanner := newCloudScanner
	newCloudScanner = func(provider string, _ credential.Resolver, _ string) (discovery.Scanner, error) {
		return fakeCloudScanner{domains: []discovery.Domain{{Provider: provider, Name: "nos.example.com", Status: "online"}}}, nil
	}
	t.Cleanup(func() { newCloudScanner = originalScanner })

	configPath := t.TempDir() + "/config.json"
	args := []string{
		"enroll", "cloud", "--provider", "tencent", "--domain", "nos.example.com",
		"--email", "ops@example.com", "--dns-provider", "cloudflare",
		"--dns-credential", "keychain:CLOUDFLARE", "--config", configPath,
	}
	var stdout, stderr strings.Builder
	if code := Run(args, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "No changes made") {
		t.Fatalf("preview code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("preview created config: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	args = append(args, "--execute")
	if code := Run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("execute code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) != 1 || cfg.Certificates[0].Domains[0] != "nos.example.com" || cfg.Certificates[0].Deployments[0].Provider != "tencent-cdn" {
		t.Fatalf("saved config = %#v", cfg)
	}
}

func TestUnknownCommandSuggestsClosestCommand(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"discovr"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), `Did you mean "discover"?`) {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestContextualHelpForDiscover(t *testing.T) {
	for _, args := range [][]string{{"help", "discover"}, {"discover", "--help"}} {
		var stdout, stderr strings.Builder
		code := Run(args, &stdout, &stderr)
		if code != 0 || !strings.Contains(stdout.String(), "tlsferry discover cloud") || !strings.Contains(stdout.String(), "--provider") {
			t.Fatalf("Run(%v) code = %d, stdout = %q, stderr = %q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestContextualHelpForReleaseSmokeCleanup(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"help", "release-smoke"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "release-smoke cleanup") || !strings.Contains(stdout.String(), "ready_for_review") {
		t.Fatalf("Run() code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
}

func TestCompletionScriptsContainCommandsAndProviderValues(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		var stdout, stderr strings.Builder
		code := Run([]string{"completion", shell}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("completion %s code = %d, stderr = %q", shell, code, stderr.String())
		}
		for _, expected := range []string{"discover", "enroll", "release-smoke", "cleanup", "cleanup-reference", "tencent", "aliyun", "qiniu", "cloudflare"} {
			if !strings.Contains(stdout.String(), expected) {
				t.Fatalf("completion %s omitted %q", shell, expected)
			}
		}
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"completion", "powershell"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "supported shells: zsh, bash, fish") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestCompletionActivationIsIdempotent(t *testing.T) {
	path := t.TempDir() + "/.zshrc"
	content := `fpath=("$HOME/.zfunc" $fpath)`
	if err := appendCompletionActivation(path, content); err != nil {
		t.Fatalf("appendCompletionActivation() error = %v", err)
	}
	if err := appendCompletionActivation(path, content); err != nil {
		t.Fatalf("second appendCompletionActivation() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "# >>> TLSFerry completion >>>") != 1 {
		t.Fatalf("activation block = %q", data)
	}
}

type fakeCredentialStore struct {
	profile string
	values  map[string]string
}

type fakeCloudScanner struct {
	domains []discovery.Domain
}

func (s fakeCloudScanner) Scan(context.Context) ([]discovery.Domain, error) {
	return s.domains, nil
}

func (s *fakeCredentialStore) Get(string) (map[string]string, error) { return s.values, nil }

func (s *fakeCredentialStore) Set(profile string, values map[string]string) error {
	s.profile = profile
	s.values = values
	return nil
}

func (s *fakeCredentialStore) Delete(string) error { return nil }
