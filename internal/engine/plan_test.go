package engine

import (
	"strings"
	"testing"

	"github.com/nonozone/TLSFerry/internal/config"
)

func TestPlanWriteTo(t *testing.T) {
	cfg := config.Config{
		RenewBefore: "720h",
		Certificates: []config.Certificate{{
			Name:    "assets-example",
			Domains: []string{"assets.example.com"},
			Issuer: config.Issuer{
				Type:        "acme",
				Challenge:   "dns-01",
				DNSProvider: "dnspod",
			},
			Deployments: []config.Deployment{{
				Provider: "tencent-cdn",
				Target:   "assets.example.com",
			}},
		}},
	}

	var output strings.Builder
	BuildPlan(cfg).Render(&output)
	for _, expected := range []string{"assets-example", "dns-01", "tencent-cdn"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("plan output %q does not contain %q", output.String(), expected)
		}
	}
}
