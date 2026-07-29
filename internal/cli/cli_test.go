package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
	"github.com/nonozone/TLSFerry/internal/discovery"
)

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

func TestCompletionScriptsContainCommandsAndProviderValues(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		var stdout, stderr strings.Builder
		code := Run([]string{"completion", shell}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("completion %s code = %d, stderr = %q", shell, code, stderr.String())
		}
		for _, expected := range []string{"discover", "enroll", "tencent", "aliyun", "qiniu", "cloudflare"} {
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
