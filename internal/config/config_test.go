package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	cfg := Config{
		RenewBefore: "720h",
		Certificates: []Certificate{{
			Name:    "assets-example",
			Domains: []string{"assets.example.com"},
			Issuer: Issuer{
				Type:         "acme",
				Email:        "ops@example.com",
				DirectoryURL: "https://acme.example/directory",
				Challenge:    "dns-01",
				DNSProvider:  "dnspod",
				Credential:   "env:TENCENTCLOUD",
			},
			Deployments: []Deployment{{
				Provider:   "tencent-cdn",
				Target:     "assets.example.com",
				Credential: "env:TENCENTCLOUD",
			}},
		}},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned an unexpected error: %v", err)
	}
}

func TestConfigRejectsDuplicateDomains(t *testing.T) {
	cfg := Config{
		RenewBefore: "720h",
		Certificates: []Certificate{{
			Name:    "duplicate",
			Domains: []string{"EXAMPLE.com", "example.com"},
			Issuer: Issuer{
				Type:         "acme",
				Email:        "ops@example.com",
				DirectoryURL: "https://acme.example/directory",
				Challenge:    "dns-01",
				DNSProvider:  "aliyun",
				Credential:   "env:ALIYUN",
			},
			Deployments: []Deployment{{
				Provider:   "aliyun-cdn",
				Target:     "example.com",
				Credential: "env:ALIYUN",
			}},
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() succeeded for duplicate domains")
	}
}

func TestConfigAllowsIssuanceWithoutDeployments(t *testing.T) {
	cfg := Config{
		RenewBefore: "720h",
		Certificates: []Certificate{{
			Name:    "standalone",
			Domains: []string{"example.com"},
			Issuer: Issuer{
				Type:         "acme",
				Email:        "ops@example.com",
				DirectoryURL: "https://acme.example/directory",
				Challenge:    "dns-01",
				DNSProvider:  "dnspod",
				Credential:   "env:TENCENTCLOUD",
			},
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() rejected an issuance-only certificate: %v", err)
	}
}

func TestSaveWritesLoadablePrivateConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg := Config{
		RenewBefore: "720h",
		Certificates: []Certificate{{
			Name: "assets", Domains: []string{"assets.example.com"},
			Issuer: Issuer{Type: "acme", Email: "ops@example.com", DirectoryURL: "https://acme.example/directory", Challenge: "dns-01", DNSProvider: "cloudflare", Credential: "keychain:CLOUDFLARE"},
		}},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil || len(loaded.Certificates) != 1 {
		t.Fatalf("Load() = %#v, %v", loaded, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
}

func TestLoadRejectsTrailingJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"renew_before":"720h","certificates":[]} {"unexpected":true}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() accepted trailing JSON")
	}
}
