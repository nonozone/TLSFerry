package config

import "testing"

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
